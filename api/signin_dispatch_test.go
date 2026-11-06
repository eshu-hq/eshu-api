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

// TestSignInPolicyNative_UnauthenticatedIs401 pins that the native route
// keeps the dest 401 shape: envelope callers get the envelope error,
// everyone else the plain error_code body, both with the bare Bearer [REDACTED]
func TestSignInPolicyNative_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := httptest.NewRequest(http.MethodPatch, "/api/v0/auth/admin/sign-in-policy", strings.NewReader(`{}`))
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

// TestSignInPolicyNative_AuthorizerRejectsNonAllScopes pins the family
// 403 the dest authorizer adds: a non-AllScopes identity never reaches
// the core. Dest has no scoped/session resolver, so the deny branch is
// unreachable in production (the shared key maps to AllScopes in core,
// which re-judges tenant scope and the identity_admin permission catalog
// as backstop); the test pins the documented shape, not a live path.
func TestSignInPolicyNative_AuthorizerRejectsNonAllScopes(t *testing.T) {
	t.Parallel()

	if !authorizeSignInPolicyAdmin(true) {
		t.Errorf("authorizeSignInPolicyAdmin(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeSignInPolicyAdmin(false) {
		t.Errorf("authorizeSignInPolicyAdmin(false) = allow, want deny (family 403)")
	}
}

// TestSignInPolicyNative_DenyBranchShapePinned pins the deny shape for
// the day dest gains a scoped/session resolver: a non-AllScopes identity
// is denied with the documented shape and the core is never reached, in
// both negotiated shapes. The branch is unreachable on the live route
// today (the route passes true; the live 403s are core's), so this pins
// the contract, not a live path.
func TestSignInPolicyNative_DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := authedRequest(http.MethodPatch, "/api/v0/auth/admin/sign-in-policy")
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		srv.signInPolicyWriteThroughAs(rec, req, false)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Accept %q: status = %d, want 403", accept, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "all-scopes operator authorization is required") {
			t.Errorf("Accept %q: body = %q, want the family-403 message", accept, body)
		}
	}
}

// TestSignInPolicyNative_WriteThroughRelaysCoreBytes pins the native
// write-through: method, path, headers (including the original
// Authorization header), and body forward to the same core route, and
// the core's status, headers, and body come back untouched across the
// pinned outcomes — 200 policy detail, the 400 require_sso guardrail
// rejection, the core 403, and the core 503 (whose Retry-After relays,
// proving dest never invents or strips the core's retry signal).
func TestSignInPolicyNative_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"updated", http.StatusOK, nil, `{"require_sso":true,"allow_local_user_creation":false,"require_mfa_for_all_users":true,"idle_timeout_seconds":900,"absolute_timeout_seconds":3600}`},
		{"guardrail rejection", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"require_sso cannot be enabled: no provider config has a passing connection test"}`},
		{"bad body", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid sign-in policy request"}`},
		{"core 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"sign-in policy store is unavailable"}`},
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

			payload := `{"require_sso":true}`
			req := authedRequest(http.MethodPatch, "/api/v0/auth/admin/sign-in-policy")
			req.Body = io.NopCloser(strings.NewReader(payload))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Fatalf("upstream hits = %d, want exactly 1 (no retry on write-through)", core.hits)
			}
			if core.lastMethod != http.MethodPatch {
				t.Errorf("upstream method = %q, want PATCH", core.lastMethod)
			}
			if core.lastPath != "/api/v0/auth/admin/sign-in-policy" {
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

// TestSignInPolicyNative_ReadsStillProxy pins that the native PATCH route
// changes nothing for the family's reads: both GET spellings still cross
// the generic byte-relay with path and method intact.
func TestSignInPolicyNative_ReadsStillProxy(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/auth/admin/sign-in-policy",
		"/api/v0/auth/sign-in-policy",
	} {
		core := &stubCore{status: http.StatusOK, body: `{"require_sso":false}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodGet, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != path || core.lastMethod != http.MethodGet {
			t.Errorf("path %q: hits = %d method = %q lastPath = %q, want the generic proxy to the read route",
				path, core.hits, core.lastMethod, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("path %q: status = %d, want 200", path, rec.Code)
		}
	}
}

// TestSignInPolicyNative_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: with no core origin configured the native route answers the
// permanent 503 backend_unavailable instead of panicking, in both
// negotiated shapes and for both spellings of the PATCH path.
func TestSignInPolicyNative_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		for _, path := range []string{
			"/api/v0/auth/admin/sign-in-policy",
			"/api/v0/auth/admin/sign-in-policy/",
		} {
			req := authedRequest(http.MethodPatch, path)
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

// TestSignInPolicyNative_BothSpellingsWriteThrough pins that both
// spellings of the PATCH path stay on the native route: method, path
// (as spelled), and body forward to the same core route.
func TestSignInPolicyNative_BothSpellingsWriteThrough(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/auth/admin/sign-in-policy",
		"/api/v0/auth/admin/sign-in-policy/",
	} {
		core := &stubCore{status: http.StatusOK, body: `{"require_sso":false}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPatch, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != path {
			t.Errorf("path %q: hits = %d lastPath = %q, want the native route to forward as spelled", path, core.hits, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("path %q: status = %d, want relayed 200", path, rec.Code)
		}
	}
}

// TestSignInPolicyNative_DeadCoreIs503WithoutRetry pins the dead-core
// shape on the native route: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count proving
// exactly one dispatch attempt (no retry on a write).
func TestSignInPolicyNative_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPatch, "/api/v0/auth/admin/sign-in-policy")
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

// TestSignInPolicyNative_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native sign-in route: a canceled caller
// gets the permanent 503 backend_unavailable with no retry, pre-cancel
// and mid-flight, with the attempt count proving a single dispatch
// attempt in both cases.
func TestSignInPolicyNative_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
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
				w.WriteHeader(http.StatusOK)
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
			req := authedRequest(http.MethodPatch, "/api/v0/auth/admin/sign-in-policy").WithContext(ctx)
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

// TestSignInPolicyNative_ParallelProbe pins that concurrent PATCH
// callers each cross the native route exactly once: N parallel requests
// yield N upstream hits and N relayed responses, proving no retry, no
// drop, and no cross-talk under concurrency.
func TestSignInPolicyNative_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"require_sso":false}`)
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
			req := authedRequest(http.MethodPatch, "/api/v0/auth/admin/sign-in-policy")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				failures[i] = fmt.Sprintf("caller %d: status = %d, want 200", i, rec.Code)
				return
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != `{"require_sso":false}` {
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

// TestSignInPolicyNative_V1AliasOrderingUnchanged pins that the v1 alias
// rewrite still lands the sign-in PATCH on its native handler with the
// v0 deprecation headers absent.
func TestSignInPolicyNative_V1AliasOrderingUnchanged(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"require_sso":false}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodPatch, "/api/v1/auth/admin/sign-in-policy")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 || core.lastPath != "/api/v0/auth/admin/sign-in-policy" {
		t.Errorf("hits = %d path = %q, want alias rewrite to the native v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want relayed 200", rec.Code)
	}
}
