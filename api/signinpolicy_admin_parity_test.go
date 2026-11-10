// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// W7-read admin sign-in-policy GET byte-parity and credential-matrix
// proof (Refs eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491,
// satellite eshu-hq/eshu-api#72). Companion to
// signinpolicy_admin_terminate_test.go: the terminated family reader
// relays core bytes exactly like the generic proxy, and the family
// gate files unresolvable identity classes on the relay lane while
// junk and credential-less callers keep the standing dest 401 with
// zero core contact.

// TestAdminSignInPolicyTerminatedMatchesRelayByteForByte pins that
// the family reader relays core bytes exactly like the generic proxy:
// same status, same body, same relayed headers — across the pinned
// outcome matrix in both negotiated shapes. The stub answers the
// pinned wire strings where core owns the text, and opaque bytes
// where the relay (not the text) is what this test pins.
func TestAdminSignInPolicyTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	outcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"plain 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"tenant_id":"tenant_a","require_sso":false}`},
		{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid sign-in policy request"}`},
		{"core all-scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`},
		{"tenant scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"admin tenant scope is required"}`},
		{"missing 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"sign-in policy not found"}`},
		{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"reader is not configured"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"reader fenced"}`},
		{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/console"}}, ""},
	}
	for _, route := range []string{
		"/api/v0/auth/admin/sign-in-policy",
		"/api/v0/auth/admin/sign-in-policy/",
	} {
		for _, outcome := range outcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				name := "GET " + route + " " + outcome.name + " " + accept
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					nativeCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
					nativeMux := http.NewServeMux()
					nativeMux.Handle("/", nativeCore.handler())
					nativeUpstream := httptest.NewServer(nativeMux)
					defer nativeUpstream.Close()

					relayCore := &stubCore{status: outcome.status, header: relayHeaders(outcome.header), body: outcome.body}
					relayMux := http.NewServeMux()
					relayMux.Handle("/", relayCore.handler())
					relayUpstream := httptest.NewServer(relayMux)
					defer relayUpstream.Close()

					build := func(target string) *http.Request {
						req := authedRequest(http.MethodGet, target)
						req.Header.Set("Accept", accept)
						req.Header.Set("X-Correlation-ID", "w7read-parity-1")
						return req
					}

					nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
					nativeRec := httptest.NewRecorder()
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, build(route))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, build(route))

					nativeRes, relayRes := nativeRec.Result(), relayRec.Result()
					if nativeRes.StatusCode != relayRes.StatusCode {
						t.Fatalf("native status = %d, relay status = %d, want identical", nativeRes.StatusCode, relayRes.StatusCode)
					}
					nativeBody, _ := io.ReadAll(nativeRes.Body)
					relayBody, _ := io.ReadAll(relayRes.Body)
					if string(nativeBody) != string(relayBody) {
						t.Errorf("native body = %q, relay body = %q, want identical", nativeBody, relayBody)
					}
					if got := nativeRes.Header.Get("Retry-After"); got != relayRes.Header.Get("Retry-After") {
						t.Errorf("native Retry-After = %q, relay Retry-After = %q, want identical", got, relayRes.Header.Get("Retry-After"))
					}
				})
			}
		}
	}
}

// TestAdminSignInPolicyRelayExplicitForUnresolvableIdentities pins
// the scope-honesty label for the lane: credential classes dest
// cannot resolve (scoped tokens, OIDC bearers, browser sessions)
// relay explicitly on the relay lane — core judges, core bytes come
// back untouched, dest invents no 401/403/404/500 for them.
func TestAdminSignInPolicyRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		header map[string]string
	}{
		{"scoped bearer", map[string]string{"Authorization": "Bearer scoped-token"}},
		{"oidc bearer", map[string]string{"Authorization": "Bearer oidc-token"}},
		{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
		{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"tenant_id":"tenant_a","require_sso":true}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			for _, target := range []string{
				"/api/v0/auth/admin/sign-in-policy",
				"/api/v0/auth/admin/sign-in-policy/",
			} {
				req := httptest.NewRequest(http.MethodGet, target, nil)
				for k, v := range tc.header {
					req.Header.Set(k, v)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Errorf("%s GET %s: upstream hits = %d, want 1 (relay explicitly, never gate-invent)", tc.name, target, core.hits)
				}
				// Reset the stub hit counter per spelling so each leg
				// proves its own dispatch.
				core.hits = 0
				if rec.Code != http.StatusOK {
					t.Errorf("%s GET %s: status = %d, want the relayed core 200 (never 401/403/404/500-invented)", tc.name, target, rec.Code)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != core.body {
					t.Errorf("%s GET %s: body = %q, want the relayed core bytes %q", tc.name, target, body, core.body)
				}
				snap := tel.snapshot()
				if snap.dispatches["sign_in_policy"][viaRelay] == nil {
					t.Errorf("%s GET %s: dispatches = %v, want a relay lane row (never invented)", tc.name, target, snap.dispatches["sign_in_policy"])
				}
			}
		})
	}
}

// TestAdminSignInPolicyJunkCookieStays401 pins the D2 follow-up
// lesson for the lane: a Cookie header without a
// session-cookie-shaped member is junk, not a browser session, and
// stays dest-401 with zero core contact.
func TestAdminSignInPolicyJunkCookieStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"tenant_id":"tenant_a"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/auth/admin/sign-in-policy",
		"/api/v0/auth/admin/sign-in-policy/",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("Cookie", "junk=1; other=abc")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s: status = %d, want the standing dest 401", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (junk cookie is not a credential)", core.hits)
	}
}

// TestAdminSignInPolicyCredentiallessStays401 pins that a
// credential-less family call gets the standing dest 401 with zero
// core contact.
func TestAdminSignInPolicyCredentiallessStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"tenant_id":"tenant_a"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/auth/admin/sign-in-policy",
		"/api/v0/auth/admin/sign-in-policy/",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s: status = %d, want the standing dest 401", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0", core.hits)
	}
}
