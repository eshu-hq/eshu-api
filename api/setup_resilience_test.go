// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSetupNative_DeadCoreIs503WithoutRetry pins the dead-core shape
// on the public carve-out: the claim POST answers the permanent 503
// backend_unavailable with no Retry-After, in both negotiated shapes,
// with the attempt count proving exactly one dispatch attempt (a
// bootstrap credential must never be replayed by a dest retry).
func TestSetupNative_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/setup/claim",
			strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`))
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := atomic.LoadInt32(&attempts); got != 1 {
			t.Errorf("Accept %q: dispatch attempts = %d, want exactly 1 (no retry)", accept, got)
		}
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Accept %q: status = %d, want 503", accept, rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Errorf("Accept %q: Retry-After = %q, want absent (permanent 503)", accept, got)
		}
		gotBody, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(gotBody), "backend_unavailable") {
			t.Errorf("Accept %q: body = %q, want backend_unavailable", accept, gotBody)
		}
	}
}

// TestSetupNative_SealedCoreSingleHit pins that a sealed core's 410
// (or a concurrent-completion 409) never triggers a dest retry: one
// POST crosses exactly once and the core's answer relays back.
func TestSetupNative_SealedCoreSingleHit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"sealed 410", http.StatusGone, `{"error_code":"gone"}`},
		{"concurrent 409", http.StatusConflict, `{"error_code":"conflict"}`},
	} {
		core := &stubCore{status: tc.status, body: tc.body}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/setup/mfa",
			strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 {
			t.Errorf("%s: upstream hits = %d, want exactly 1 (no dest retry)", tc.name, core.hits)
		}
		if rec.Code != tc.status {
			t.Errorf("%s: status = %d, want relayed %d", tc.name, rec.Code, tc.status)
		}
		gotBody, _ := io.ReadAll(rec.Result().Body)
		if string(gotBody) != tc.body {
			t.Errorf("%s: body = %q, want the core bytes %q", tc.name, gotBody, tc.body)
		}
	}
}

// TestSetupNative_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the carve-out: a pre-canceled claim
// POST fails closed with the permanent 503 and a single dispatch
// attempt — the credential is not replayed after the caller left.
func TestSetupNative_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	var attempts int32
	srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
	handler := srv.Handler(DefaultV0Sunset)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/setup/claim",
		strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`)).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("dispatch attempts = %d, want exactly 1 (no retry on cancel)", got)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 after the caller left", rec.Code)
	}
	gotBody, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(gotBody), "backend_unavailable") {
		t.Errorf("body = %q, want backend_unavailable", gotBody)
	}
}

// TestSetupNative_CredentialBodyNeverLogged pins the body-non-logging
// contract: bootstrap passwords, new passwords, and core-issued
// recovery codes cross the carve-out without ever reaching the
// structured log. The log carries one line per request with only the
// closed labels — method, family, lane, status — never a secret.
func TestSetupNative_CredentialBodyNeverLogged(t *testing.T) {
	t.Parallel()

	const bootstrapPassword = "bootstrap-p-39f2c1"
	const newPassword = "fresh-secret-16!!-7d4a"
	const recoveryCode = "rc-one-time-9e55aa"

	core := &stubCore{
		status: http.StatusOK,
		body:   `{"status":"completed","recovery_codes":["` + recoveryCode + `"]}`,
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	var logs strings.Builder
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = NewTelemetry(slog.New(slog.NewJSONHandler(&logs, nil)))
	handler := srv.Handler(DefaultV0Sunset)

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v0/auth/setup-state", ""},
		{
			http.MethodPost, "/api/v0/auth/setup/claim",
			`{"username":"bootstrap-u","password":"` + bootstrapPassword + `"}`,
		},
		{
			http.MethodPost, "/api/v0/auth/setup/admin",
			`{"username":"bootstrap-u","password":"` + bootstrapPassword + `","new_password":"` + newPassword + `"}`,
		},
		{
			http.MethodPost, "/api/v0/auth/setup/mfa",
			`{"username":"bootstrap-u","password":"` + bootstrapPassword + `"}`,
		},
	}
	for _, tc := range requests {
		var body io.Reader
		if tc.body != "" {
			body = strings.NewReader(tc.body)
		}
		req := httptest.NewRequest(tc.method, tc.path, body)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: status = %d, want relayed 200", tc.method, tc.path, rec.Code)
		}
	}

	for _, secret := range []string{bootstrapPassword, newPassword, recoveryCode, skeletonTestKey} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("structured logs contain %q, want no credential or recovery-code bytes in telemetry", secret)
		}
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != len(requests) {
		t.Errorf("log lines = %d, want %d (one closed-label line per request)", len(lines), len(requests))
	}
}

// TestSetupNative_ParallelWriteThrough pins concurrent carve-out
// calls: N parallel claim POSTs cross as N upstream hits and every
// caller observes its own relayed answer.
func TestSetupNative_ParallelWriteThrough(t *testing.T) {
	t.Parallel()

	const parallel = 8
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"claimed"}`)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	var wg sync.WaitGroup
	errs := make(chan string, parallel)
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/setup/claim",
				strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				errs <- "status is not relayed 200"
				return
			}
			gotBody, _ := io.ReadAll(rec.Result().Body)
			if string(gotBody) != `{"status":"claimed"}` {
				errs <- "body is not the relayed core bytes"
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("parallel claims did not finish: %d upstream hits", hits.Load())
	}
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := hits.Load(); got != parallel {
		t.Errorf("upstream hits = %d, want %d (one forward per caller, no coalescing)", got, parallel)
	}
}

// TestSetupNative_SealedStateSuite pins the no-dest-gate contract
// against a sealed core: the three mutations answer the core's own
// 410 bytes (dest synthesizes none) and setup-state answers 200 with
// needs_setup:false. The sealed body is core-shaped — dest's own
// error writer is never in the path.
func TestSetupNative_SealedStateSuite(t *testing.T) {
	t.Parallel()

	const sealedBody = `{"error_code":"gone","message":"first-run setup is no longer available; this instance already has an identity","correlation_id":"seal-9"}`

	sealed := &stubCore{status: http.StatusGone, body: sealedBody}
	sealedMux := http.NewServeMux()
	sealedMux.Handle("/", sealed.handler())
	sealedUpstream := httptest.NewServer(sealedMux)
	defer sealedUpstream.Close()

	sealedHandler := NewServer(sealedUpstream.URL, skeletonTestKey).Handler(DefaultV0Sunset)

	for _, spelling := range setupSpellings() {
		if spelling.method == http.MethodGet {
			continue
		}
		req := httptest.NewRequest(spelling.method, spelling.path,
			strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`))
		rec := httptest.NewRecorder()
		sealedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusGone {
			t.Errorf("%s %q: status = %d, want the relayed sealed 410",
				spelling.method, spelling.path, rec.Code)
		}
		gotBody, _ := io.ReadAll(rec.Result().Body)
		if string(gotBody) != sealedBody {
			t.Errorf("%s %q: body = %q, want the sealed core bytes (no dest-synthesized 410)",
				spelling.method, spelling.path, gotBody)
		}
	}

	open := &stubCore{status: http.StatusOK, body: `{"needs_setup":false,"bootstrap_mode":"generated"}`}
	openMux := http.NewServeMux()
	openMux.Handle("/", open.handler())
	openUpstream := httptest.NewServer(openMux)
	defer openUpstream.Close()

	openHandler := NewServer(openUpstream.URL, skeletonTestKey).Handler(DefaultV0Sunset)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/setup-state", nil)
	rec := httptest.NewRecorder()
	openHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("sealed setup-state: status = %d, want relayed 200", rec.Code)
	}
	gotBody, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(gotBody), `"needs_setup":false`) {
		t.Errorf("sealed setup-state: body = %q, want needs_setup:false", gotBody)
	}
}

// TestSetupNative_Indistinguishable401 pins that a wrong bootstrap
// credential relays the core's single 401 shape byte-for-byte, with
// or without a dest credential on the request: dest adds no signal
// that distinguishes missing, consumed, or mismatched credentials.
func TestSetupNative_Indistinguishable401(t *testing.T) {
	t.Parallel()

	const deniedBody = `{"error_code":"unauthenticated","message":"bootstrap credential is invalid or expired","correlation_id":"deny-7"}`

	for _, destAuth := range []string{"", "Bearer bogus"} {
		core := &stubCore{status: http.StatusUnauthorized, body: deniedBody}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		for _, path := range []string{
			"/api/v0/auth/setup/claim",
			"/api/v0/auth/setup/admin",
			"/api/v0/auth/setup/mfa",
		} {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				req := httptest.NewRequest(http.MethodPost, path,
					strings.NewReader(`{"username":"bootstrap-u","password":"wrong"}`))
				req.Header.Set("Accept", accept)
				if destAuth != "" {
					req.Header.Set("Authorization", destAuth)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusUnauthorized {
					t.Errorf("dest auth %q %s Accept %q: status = %d, want relayed 401",
						destAuth, path, accept, rec.Code)
				}
				gotBody, _ := io.ReadAll(rec.Result().Body)
				if string(gotBody) != deniedBody {
					t.Errorf("dest auth %q %s Accept %q: body = %q, want the indistinguishable core 401 bytes",
						destAuth, path, accept, gotBody)
				}
				if got := rec.Header().Get("WWW-Authenticate"); got != "" {
					t.Errorf("dest auth %q %s Accept %q: WWW-Authenticate = %q, want empty (core owns the 401 shape)",
						destAuth, path, accept, got)
				}
			}
		}
	}
}

// TestSetupNative_BothSpellingsWriteThrough pins that both spellings
// of every setup path stay on the native route: method and path (as
// spelled) forward to the same core route without a credential.
func TestSetupNative_BothSpellingsWriteThrough(t *testing.T) {
	t.Parallel()

	for _, spelling := range setupSpellings() {
		core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		var body io.Reader
		if spelling.method != http.MethodGet {
			body = strings.NewReader(`{}`)
		}
		req := httptest.NewRequest(spelling.method, spelling.path, body)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != spelling.path {
			t.Errorf("%s %q: hits = %d lastPath = %q, want the native route to forward as spelled",
				spelling.method, spelling.path, core.hits, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("%s %q: status = %d, want relayed 200", spelling.method, spelling.path, rec.Code)
		}
	}
}

// TestSetupNative_UnconfiguredOriginFailsClosed pins the fail-closed
// origin on the public carve-out: with no core origin configured every
// native spelling answers the permanent 503 backend_unavailable
// instead of panicking — still with no credential — in both
// negotiated shapes.
func TestSetupNative_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range setupSpellings() {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			var body io.Reader
			if spelling.method != http.MethodGet {
				body = strings.NewReader(`{}`)
			}
			req := httptest.NewRequest(spelling.method, spelling.path, body)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("%s %q Accept %q: panicked with %v, want 503", spelling.method, spelling.path, accept, recovered)
					}
				}()
				handler.ServeHTTP(rec, req)
			}()

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s %q Accept %q: status = %d, want 503", spelling.method, spelling.path, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%s %q Accept %q: Retry-After = %q, want absent (permanent 503)", spelling.method, spelling.path, accept, got)
			}
			gotBody, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(gotBody), "backend_unavailable") {
				t.Errorf("%s %q Accept %q: body = %q, want backend_unavailable", spelling.method, spelling.path, accept, gotBody)
			}
		}
	}
}
