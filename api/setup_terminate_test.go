// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// D10 setup-state termination proof box (Refs eshu-hq/eshu#9669, part
// of #6707). GET /api/v0/auth/setup-state terminates dest-side via
// the SetupNeeded port: the seal signal comes from a single core
// fetch, dest synthesizes the 200, and every failure closes to the
// permanent local 503 (never a 410). The three credential POSTs stay
// tier-C write-through and keep their C-c1 proofs in
// setup_dispatch_test.go and setup_resilience_test.go unmodified.

// TestSetupTerminated_OpenStateSemanticParity pins the terminated
// GET shape: open and sealed cores each answer one dest-synthesized
// 200 whose decoded keys match the core shape (needs_setup plus the
// bootstrap_mode echo), in both negotiated shapes, with exactly one
// upstream hit carrying the forwarded query and no credential.
func TestSetupTerminated_OpenStateSemanticParity(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		coreBody  string
		wantSetup bool
		wantMode  string
	}{
		{"open", `{"needs_setup":true,"bootstrap_mode":"generated"}`, true, "generated"},
		{"sealed", `{"needs_setup":false,"bootstrap_mode":"generated"}`, false, "generated"},
		{"sealed empty mode", `{"needs_setup":false,"bootstrap_mode":""}`, false, ""},
	} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{status: http.StatusOK, body: tc.coreBody}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/setup-state?wizard=1", nil)
			req.Header.Set("Accept", accept)
			req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("%s Accept %q: status = %d, want terminated 200", tc.name, accept, rec.Code)
			}
			var got struct {
				NeedsSetup    bool   `json:"needs_setup"`
				BootstrapMode string `json:"bootstrap_mode"`
			}
			raw, _ := io.ReadAll(rec.Result().Body)
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Errorf("%s Accept %q: decode %q: %v", tc.name, accept, raw, err)
			}
			if got.NeedsSetup != tc.wantSetup || got.BootstrapMode != tc.wantMode {
				t.Errorf("%s Accept %q: state = %+v, want needs_setup=%v mode=%q",
					tc.name, accept, got, tc.wantSetup, tc.wantMode)
			}
			if core.hits != 1 {
				t.Errorf("%s Accept %q: upstream hits = %d, want exactly 1 (no dest retry, no cache)",
					tc.name, accept, core.hits)
			}
			if core.lastPath != "/api/v0/auth/setup-state" || core.lastQuery != "wizard=1" {
				t.Errorf("%s Accept %q: upstream = %s?%s, want the canonical core route and query",
					tc.name, accept, core.lastPath, core.lastQuery)
			}
			if core.lastAuth != "" {
				t.Errorf("%s Accept %q: upstream Authorization = %q, want empty (no credential crosses)",
					tc.name, accept, core.lastAuth)
			}
		}
	}
}

// TestSetupTerminated_SealedStateBox pins the sealed box end to end:
// against a sealed core the three credential POSTs keep their
// relayed 410s (tier-C write-through, dest synthesizes none) while
// the terminated GET answers the dest-synthesized 200 with
// needs_setup:false.
func TestSetupTerminated_SealedStateBox(t *testing.T) {
	t.Parallel()

	const sealedBody = `{"error_code":"gone","message":"first-run setup is no longer available; this instance already has an identity","correlation_id":"seal-9"}`
	const sealedState = `{"needs_setup":false,"bootstrap_mode":"generated"}`

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, sealedState)
			return
		}
		w.WriteHeader(http.StatusGone)
		_, _ = io.WriteString(w, sealedBody)
	}))
	defer upstream.Close()

	handler := NewServer(upstream.URL, skeletonTestKey).Handler(DefaultV0Sunset)

	for _, spelling := range setupSpellings() {
		var body io.Reader
		if spelling.method != http.MethodGet {
			body = strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`)
		}
		req := httptest.NewRequest(spelling.method, spelling.path, body)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if spelling.method == http.MethodGet {
			if rec.Code != http.StatusOK {
				t.Errorf("GET %q: status = %d, want terminated 200", spelling.path, rec.Code)
			}
			gotBody, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(gotBody), `"needs_setup":false`) {
				t.Errorf("GET %q: body = %q, want needs_setup:false", spelling.path, gotBody)
			}
			continue
		}
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
	if got := hits.Load(); got != int32(len(setupSpellings())) {
		t.Errorf("upstream hits = %d, want %d (one fetch per spelling)", got, len(setupSpellings()))
	}
}

// TestSetupTerminated_CoreErrorFailsClosed pins the fail-closed
// mapping: any non-200 from the core fetch — the 500 store failure,
// the impossible-but-defensive 410 — answers the permanent local 503
// in both shapes. Dest never relays a core failure and never emits a
// 410 on the terminated route.
func TestSetupTerminated_CoreErrorFailsClosed(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		coreStatus int
		coreBody   string
	}{
		{"store failure is 503", http.StatusInternalServerError, `{"error_code":"server_failure"}`},
		{"core gone is 503 never 410", http.StatusGone, `{"error_code":"gone"}`},
		{"malformed body is 503", http.StatusOK, `not-json{{`},
		{"missing seal signal is 503", http.StatusOK, `{"bootstrap_mode":"generated"}`},
	} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{status: tc.coreStatus, body: tc.coreBody}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			tel := NewTelemetry(nil)
			srv.WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/setup-state", nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s Accept %q: status = %d, want fail-closed 503",
					tc.name, accept, rec.Code)
			}
			if rec.Code == http.StatusGone {
				t.Errorf("%s Accept %q: terminated route answered 410, want never-410", tc.name, accept)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%s Accept %q: Retry-After = %q, want absent (permanent 503)", tc.name, accept, got)
			}
			gotBody, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(gotBody), "backend_unavailable") {
				t.Errorf("%s Accept %q: body = %q, want backend_unavailable", tc.name, accept, gotBody)
			}
			if core.hits != 1 {
				t.Errorf("%s Accept %q: upstream hits = %d, want exactly 1 (no dest retry)",
					tc.name, accept, core.hits)
			}
			snap := tel.snapshot()
			if snap.dispatches["setup"]["native"]["local_503"] != 1 {
				t.Errorf("%s Accept %q: dispatch rows = %v, want one setup/native/local_503 (extended, not renamed)",
					tc.name, accept, snap.dispatches["setup"])
			}
		}
	}
}

// TestSetupTerminated_DeadCoreIs503WithoutRetry pins the dead-core
// shape on the terminated GET: the permanent 503 in both shapes with
// exactly one dispatch attempt.
func TestSetupTerminated_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/setup-state", nil)
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

// TestSetupTerminated_CanceledContextFailsClosedWithoutRetry pins
// cancellation on the terminated GET: a pre-canceled fetch fails
// closed with the permanent 503 and a single dispatch attempt.
func TestSetupTerminated_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	var attempts int32
	srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
	handler := srv.Handler(DefaultV0Sunset)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/setup-state", nil).WithContext(ctx)
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

// TestSetupTerminated_UnconfiguredOriginFailsClosed pins the
// fail-closed origin on the terminated GET: no core origin answers
// the permanent 503 with no credential, in both shapes, on both
// spellings — never a 410.
func TestSetupTerminated_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{"/api/v0/auth/setup-state", "/api/v0/auth/setup-state/"} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("GET %q Accept %q: panicked with %v, want 503", path, accept, recovered)
					}
				}()
				handler.ServeHTTP(rec, req)
			}()

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("GET %q Accept %q: status = %d, want 503 (never 410)", path, accept, rec.Code)
			}
			gotBody, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(gotBody), "backend_unavailable") {
				t.Errorf("GET %q Accept %q: body = %q, want backend_unavailable", path, accept, gotBody)
			}
		}
	}
}

// TestSetupTerminated_SlashTwinUsesCanonicalFetch pins that the
// trailing-slash twin terminates on the same fetch: dest matched the
// route, so the core sees the single canonical path — never a split
// dispatch, never a twin 404.
func TestSetupTerminated_SlashTwinUsesCanonicalFetch(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"needs_setup":true,"bootstrap_mode":"generated"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/setup-state/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("twin status = %d, want terminated 200", rec.Code)
	}
	if core.hits != 1 || core.lastPath != "/api/v0/auth/setup-state" {
		t.Errorf("hits = %d lastPath = %q, want one fetch of the canonical core path",
			core.hits, core.lastPath)
	}
	gotBody, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(gotBody), `"needs_setup":true`) {
		t.Errorf("twin body = %q, want needs_setup:true", gotBody)
	}
}

// TestSetupTerminated_SlashTwinSubtree pins the twin-subtree
// behavior the carve-out keeps: a deeper path under the slash twin
// still terminates on the same single canonical fetch — dest matched
// the route, so the core never sees the deeper spelling.
func TestSetupTerminated_SlashTwinSubtree(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"needs_setup":true,"bootstrap_mode":"generated"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/setup-state/extra", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("subtree status = %d, want terminated 200", rec.Code)
	}
	if core.hits != 1 || core.lastPath != "/api/v0/auth/setup-state" {
		t.Errorf("hits = %d lastPath = %q, want one fetch of the canonical core path",
			core.hits, core.lastPath)
	}
}

// TestSetupTerminated_V1AliasTerminates pins that the v1 alias
// rewrite runs ahead of the terminated GET: the v1 spelling serves
// from the port with a single fetch of the rewritten v0 core path.
func TestSetupTerminated_V1AliasTerminates(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"needs_setup":true,"bootstrap_mode":"generated"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/setup-state", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("v1 setup-state status = %d, want terminated 200", rec.Code)
	}
	if core.hits != 1 || core.lastPath != "/api/v0/auth/setup-state" {
		t.Errorf("hits = %d lastPath = %q, want one fetch of the rewritten v0 path",
			core.hits, core.lastPath)
	}
}
