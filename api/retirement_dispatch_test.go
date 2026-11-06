// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRetirementNative_UnauthenticatedIs401 pins that the native route
// keeps the dest 401 shape: envelope callers get the envelope error,
// everyone else the plain error_code body, both with the bare Bearer
// challenge and no WWW-Authenticate parameters.
func TestRetirementNative_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v0/admin/repository-retirements", strings.NewReader(`{}`))
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Accept %q: status = %d, want 401", accept, rec.Code)
		}
		if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Errorf("Accept %q: WWW-Authenticate = %q, want bare Bearer", accept, got)
		}
	}
}

// TestRetirementNative_AuthorizerRejectsNonAllScopes pins the family
// 403 the dest authorizer adds: a non-AllScopes identity never reaches
// the core. Dest has no scoped/session resolver, so the deny branch is
// unreachable in production (the shared key maps to AllScopes in core,
// which re-judges admin scope as backstop); the test pins the documented
// shape, not a live path.
func TestRetirementNative_AuthorizerRejectsNonAllScopes(t *testing.T) {
	t.Parallel()

	if !authorizeRetirementAdmin(true) {
		t.Errorf("authorizeRetirementAdmin(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeRetirementAdmin(false) {
		t.Errorf("authorizeRetirementAdmin(false) = allow, want deny (family 403)")
	}
}

// TestRetirementNative_DenyBranchShapePinned pins the deny shape for
// the day dest gains a scoped/session resolver: a non-AllScopes identity
// is denied with the documented shape and the core is never reached, in
// both negotiated shapes. The branch is unreachable on the live route
// today (the route passes true; the live 403s are core's), so this pins
// the contract, not a live path.
func TestRetirementNative_DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := authedRequest(http.MethodPost, "/api/v0/admin/repository-retirements")
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		srv.retirementWriteThroughAs(rec, req, false)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Accept %q: status = %d, want 403", accept, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "all-scopes operator authorization is required") {
			t.Errorf("Accept %q: body = %q, want the family-403 message", accept, body)
		}
	}
}

// TestRetirementNative_WriteThroughRelaysCoreBytes pins the native
// write-through: method, path, headers (including the original
// Authorization header), and body forward to the same core route, and
// the core's status, headers, and body come back untouched across the
// pinned outcomes — 202 accepted, the 400 validation rejections, the
// core 403, the 409 fence/already-open refusals, and the core 503 (whose
// Retry-After relays, proving dest never invents or strips the core's
// retry signal).
func TestRetirementNative_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"accepted", http.StatusAccepted, nil, `{"status":"accepted","retirements":[{"retirement_id":"rr_abc","repository_id":"r1","scope_id":"s1","outcome":"completed"}],"idempotency_key":"k1","duplicate":false}`},
		{"empty selector list", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"no retirement recorded; want 1 to 25 repositories, got 0"}`},
		{"missing reason", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"reason is required and must explain why the retirement is safe"}`},
		{"missing idempotency key", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"idempotency_key is required to make retirement safe under retries"}`},
		{"core 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"repository retirement requires an admin (all-scopes) token"}`},
		{"already open", http.StatusConflict, nil, `{"error":"Conflict","detail":"repository retirement refused: a retirement is already open for this repository under a different idempotency key"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"repository retirement request failed"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			payload := `{"repositories":["r1"],"reason":"decommissioned","idempotency_key":"k1"}`
			req := authedRequest(http.MethodPost, "/api/v0/admin/repository-retirements")
			req.Body = io.NopCloser(strings.NewReader(payload))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Fatalf("upstream hits = %d, want exactly 1 (no retry on write-through)", core.hits)
			}
			if core.lastMethod != http.MethodPost {
				t.Errorf("upstream method = %q, want POST", core.lastMethod)
			}
			if core.lastPath != "/api/v0/admin/repository-retirements" {
				t.Errorf("upstream path = %q, want the core route", core.lastPath)
			}
			if core.lastBody != payload {
				t.Errorf("upstream body = %q, want the caller bytes", core.lastBody)
			}
			if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
				t.Errorf("upstream Authorization = %q, want the forwarded Bearer [REDACTED]", core.lastAuth)
			}
			if rec.Code != tc.status {
				t.Errorf("status = %d, want relayed %d", rec.Code, tc.status)
			}
			if tc.header != nil {
				if got := rec.Header().Get("Retry-After"); got != tc.header.Get("Retry-After") {
					t.Errorf("Retry-After = %q, want the core's value relayed", got)
				}
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != tc.body {
				t.Errorf("body = %q, want the core bytes %q", body, tc.body)
			}
		})
	}
}

// TestRetirementNative_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: with no core origin configured the native route answers the
// permanent 503 backend_unavailable instead of panicking, in both
// negotiated shapes and for both spellings of the POST path.
func TestRetirementNative_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		for _, path := range []string{
			"/api/v0/admin/repository-retirements",
			"/api/v0/admin/repository-retirements/",
		} {
			req := authedRequest(http.MethodPost, path)
			if accept != "" {
				req.Header.Set("Accept", accept)
			}
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("Accept %q path %q: panicked with %v, want 503", accept, path, recovered)
					}
				}()
				handler.ServeHTTP(rec, req)
			}()

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("Accept %q path %q: status = %d, want 503", accept, path, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("Accept %q path %q: Retry-After = %q, want absent (permanent 503)", accept, path, got)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), "backend_unavailable") {
				t.Errorf("Accept %q path %q: body = %q, want backend_unavailable", accept, path, body)
			}
		}
	}
}

// TestRetirementNative_BothSpellingsWriteThrough pins that both
// spellings of the POST path stay on the native route: method, path
// (as spelled), and body forward to the same core route.
func TestRetirementNative_BothSpellingsWriteThrough(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/admin/repository-retirements",
		"/api/v0/admin/repository-retirements/",
	} {
		core := &stubCore{status: http.StatusAccepted, body: `{"status":"accepted"}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != path {
			t.Errorf("path %q: hits = %d lastPath = %q, want the native route to forward as spelled", path, core.hits, core.lastPath)
		}
		if rec.Code != http.StatusAccepted {
			t.Errorf("path %q: status = %d, want relayed 202", path, rec.Code)
		}
	}
}

// TestRetirementNative_DeadCoreIs503WithoutRetry pins the dead-core
// shape on the native route: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count proving
// exactly one dispatch attempt (no retry on a write).
func TestRetirementNative_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, "/api/v0/admin/repository-retirements")
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
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
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "backend_unavailable") {
			t.Errorf("Accept %q: body = %q, want backend_unavailable", accept, body)
		}
	}
}

// TestRetirementNative_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native retirement route: a canceled caller
// gets the permanent 503 backend_unavailable with no retry, pre-cancel
// and mid-flight, with the attempt count proving a single dispatch
// attempt in both cases.
func TestRetirementNative_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cancel string // "before" serves a pre-canceled context; "during" cancels mid-flight
	}{
		{"pre-cancel", "before"},
		{"mid-flight", "during"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(500 * time.Millisecond)
				w.WriteHeader(http.StatusAccepted)
			}))
			defer slow.Close()

			srv := NewServer(slow.URL, skeletonTestKey)
			var attempts int32
			srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
			handler := srv.Handler(DefaultV0Sunset)

			ctx, cancel := context.WithCancel(context.Background())
			if tc.cancel == "before" {
				cancel()
			} else {
				defer cancel()
				go func() {
					time.Sleep(100 * time.Millisecond)
					cancel()
				}()
			}
			req := authedRequest(http.MethodPost, "/api/v0/admin/repository-retirements").WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := atomic.LoadInt32(&attempts); got != 1 {
				t.Errorf("dispatch attempts = %d, want exactly 1 (no retry)", got)
			}
			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), "backend_unavailable") {
				t.Errorf("body = %q, want backend_unavailable", body)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
			}
		})
	}
}

// TestRetirementNative_ParallelProbe pins that concurrent POST
// callers each cross the native route exactly once: N parallel requests
// yield N upstream hits and N relayed responses, proving no retry, no
// drop, and no cross-talk under concurrency.
func TestRetirementNative_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"status":"accepted"}`)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	const callers = 16
	var wg sync.WaitGroup
	failures := make([]string, callers)
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := authedRequest(http.MethodPost, "/api/v0/admin/repository-retirements")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusAccepted {
				failures[i] = fmt.Sprintf("caller %d: status = %d, want 202", i, rec.Code)
				return
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != `{"status":"accepted"}` {
				failures[i] = fmt.Sprintf("caller %d: body = %q, want the relayed core bytes", i, body)
			}
		}(i)
	}
	wg.Wait()

	for _, failure := range failures {
		if failure != "" {
			t.Error(failure)
		}
	}
	if got := hits.Load(); got != callers {
		t.Errorf("upstream hits = %d, want %d (one write-through per caller, no retry)", got, callers)
	}
}

// TestRetirementNative_V1AliasOrderingUnchanged pins that the v1 alias
// rewrite still lands the retirement POST on its native handler with the
// v0 deprecation headers absent.
func TestRetirementNative_V1AliasOrderingUnchanged(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusAccepted, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodPost, "/api/v1/admin/repository-retirements")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 || core.lastPath != "/api/v0/admin/repository-retirements" {
		t.Errorf("hits = %d path = %q, want alias rewrite to the native v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want relayed 202", rec.Code)
	}
}
