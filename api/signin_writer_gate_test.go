// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// W7-writers admin sign-in-policy PATCH termination proof (Refs
// eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491, satellite
// eshu-hq/eshu-api#114; core writer record eshu-hq/eshu#10531,
// SignInPolicyAdmission v0). TDD record: this suite failed RED before
// the family writer gate landed (shared-key PATCH legs rode
// RequireAuth with no dispatchOutcome/authorizer filing, and foreign
// credential classes took the dest 401 instead of the explicit relay),
// and is green with it. The PATCH terminates natively for shared-key
// callers on the family gate while unresolvable identity classes relay
// explicitly; credential-less and junk-cookie callers keep the standing
// dest 401 with zero core contact. Serving semantics match the
// tier-C write-through the route rides today bit for bit (same
// forward, same bytes) — the lane changes ownership and proof, not
// behavior for the live class.

// TestSignInPolicyWriter_TerminatesAheadOfRelay pins that the admin
// PATCH is an explicit family registration ahead of the generic proxy:
// a shared-key call files its dispatch on the native lane (viaNative)
// with one family-authorizer allow judgment per spelling. This test
// is the swap-mutant detector for dispatch: if the family
// registration is removed, the leg falls back to the relay and files
// relay instead of native, and this test goes RED.
func TestSignInPolicyWriter_TerminatesAheadOfRelay(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"require_sso":false}`}
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
		req := authedRequest(http.MethodPatch, target)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH %s: status = %d, want 200 from the stub core", target, rec.Code)
		}
	}
	if core.hits != 2 {
		t.Fatalf("upstream hits = %d, want 2 (exact path plus twin, one forward each)", core.hits)
	}
	snap := tel.snapshot()
	if got := snap.authorizer["sign_in_policy"][decisionAllow]; got != 2 {
		t.Errorf("authorizer[sign_in_policy][allow] = %d, want 2 (exact path plus twin)", got)
	}
	lanes, ok := snap.dispatches["sign_in_policy"]
	if !ok {
		t.Fatal("family sign_in_policy has no dispatch rows, want the explicit family route ahead of relay")
	}
	if lanes[viaNative] == nil {
		t.Errorf("family sign_in_policy dispatches = %v, want a native lane row for the shared-key family call", lanes)
	}
	if got := lanes[viaNative][outcomeCoreOK]; got != 2 {
		t.Errorf("family sign_in_policy native core_ok = %d, want 2", got)
	}
}

// TestSignInPolicyWriter_MatchesRelayByteForByte pins that the family
// writer relays core bytes exactly like the generic proxy: same
// status, same body, same relayed headers — across the pinned outcome
// matrix in both negotiated shapes. The stub answers the pinned wire
// strings where core owns the text, and opaque bytes where the relay
// (not the text) is what this test pins.
func TestSignInPolicyWriter_MatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	payload := `{"require_sso":true}`
	outcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"updated 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"require_sso":true,"allow_local_user_creation":false,"require_mfa_for_all_users":true,"idle_timeout_seconds":900,"absolute_timeout_seconds":3600}`},
		{"guardrail 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"require_sso cannot be enabled: no provider config has a passing connection test"}`},
		{"timeout ordering 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"absolute_timeout_seconds must not be shorter than idle_timeout_seconds"}`},
		{"malformed 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid sign-in policy request"}`},
		{"permission catalog 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"identity_admin.sign_in_policy_update permission is required"}`},
		{"admin scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`},
		{"tenant scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"admin tenant scope is required"}`},
		{"core 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"sign-in policy update failed"}`},
		{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"sign-in policy store is unavailable"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"sign-in policy store is fenced"}`},
		{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/console"}}, ""},
	}
	for _, route := range []string{
		"/api/v0/auth/admin/sign-in-policy",
		"/api/v0/auth/admin/sign-in-policy/",
	} {
		for _, outcome := range outcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				name := "PATCH " + route + " " + outcome.name + " " + accept
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
						req := authedRequest(http.MethodPatch, target)
						req.Body = io.NopCloser(strings.NewReader(payload))
						req.Header.Set("Accept", accept)
						req.Header.Set("X-Correlation-ID", "w7write-parity-1")
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
					for _, header := range []string{"Content-Type", "Retry-After", "Location", "Cache-Control", "X-Correlation-ID"} {
						if nativeRes.Header.Get(header) != relayRes.Header.Get(header) {
							t.Errorf("header %s: native = %q, relay = %q, want identical",
								header, nativeRes.Header.Get(header), relayRes.Header.Get(header))
						}
					}
					if nativeCore.hits != 1 || relayCore.hits != 1 {
						t.Errorf("upstream hits native = %d relay = %d, want exactly 1 each (no retry on terminated writers)",
							nativeCore.hits, relayCore.hits)
					}
					if nativeCore.lastAuth == "" || nativeCore.lastAuth != relayCore.lastAuth {
						t.Errorf("upstream Authorization native = %q relay = %q, want the identical forwarded credential",
							nativeCore.lastAuth, relayCore.lastAuth)
					}
				})
			}
		}
	}
}

// TestSignInPolicyWriterGate_ForeignCredentialRelaysExplicit pins the
// scope-honesty label for the writer lane: credential classes dest
// cannot resolve (scoped tokens, OIDC bearers, browser sessions)
// relay explicitly on the relay lane — core judges, core bytes come
// back untouched, dest invents no 401/403/404/500 for them.
func TestSignInPolicyWriterGate_ForeignCredentialRelaysExplicit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		header map[string]string
	}{
		{"scoped bearer", map[string]string{"Authorization": "Bearer scoped-opaque"}},
		{"oidc bearer", map[string]string{"Authorization": "Bearer oidc-opaque"}},
		{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
		{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"require_sso":true}`}
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
				req := httptest.NewRequest(http.MethodPatch, target, strings.NewReader(`{"require_sso":true}`))
				for k, v := range tc.header {
					req.Header.Set(k, v)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Errorf("%s PATCH %s: upstream hits = %d, want 1 (relay explicitly, never gate-invent)", tc.name, target, core.hits)
				}
				// Reset the stub hit counter per spelling so each leg
				// proves its own dispatch.
				core.hits = 0
				if rec.Code != http.StatusOK {
					t.Errorf("%s PATCH %s: status = %d, want the relayed core 200 (never 401/403/404/500-invented)", tc.name, target, rec.Code)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != core.body {
					t.Errorf("%s PATCH %s: body = %q, want the relayed core bytes %q", tc.name, target, body, core.body)
				}
				snap := tel.snapshot()
				if snap.dispatches["sign_in_policy"][viaRelay] == nil {
					t.Errorf("%s PATCH %s: dispatches = %v, want a relay lane row (never invented)", tc.name, target, snap.dispatches["sign_in_policy"])
				}
			}
		})
	}
}

// TestSignInPolicyWriterGate_JunkCookieStays401 pins that a Cookie
// header without a session-cookie-shaped member is junk, not a
// browser session, and stays dest-401 with zero core contact on the
// writer lane.
func TestSignInPolicyWriterGate_JunkCookieStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"require_sso":false}`}
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
		req := httptest.NewRequest(http.MethodPatch, target, strings.NewReader(`{}`))
		req.Header.Set("Cookie", "junk=1; other=abc")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("PATCH %s: status = %d, want the standing dest 401", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (junk cookie is not a credential)", core.hits)
	}
}

// TestSignInPolicyWriterGate_CredentiallessStays401 pins that a
// credential-less writer call gets the standing dest 401 with zero
// core contact.
func TestSignInPolicyWriterGate_CredentiallessStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"require_sso":false}`}
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
		req := httptest.NewRequest(http.MethodPatch, target, strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("PATCH %s: status = %d, want the standing dest 401", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0", core.hits)
	}
}

// TestSignInPolicyWriterGate_EmptyKeyFailsClosed pins that an empty
// configured key fails closed like RequireAuth: even a well-formed
// shared-key-shaped caller gets the standing 401.
func TestSignInPolicyWriterGate_EmptyKeyFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", "")
	req := authedRequest(http.MethodPatch, "/api/v0/auth/admin/sign-in-policy")
	rec := httptest.NewRecorder()
	srv.Handler(DefaultV0Sunset).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want the standing 401 (empty configured key fails closed)", rec.Code)
	}
}

// TestSignInPolicyWriter_ExplicitRelayNilCore503 pins the fail-closed
// origin on the explicit-relay path: an unresolvable credential class
// with no core origin configured answers the permanent 503
// backend_unavailable with no Retry-After.
func TestSignInPolicyWriter_ExplicitRelayNilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	req := httptest.NewRequest(http.MethodPatch, "/api/v0/auth/admin/sign-in-policy", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer scoped-opaque")
	rec := httptest.NewRecorder()
	srv.signInPolicyRelayExplicit(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want the permanent 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent, never retry)", got)
	}
}

// TestSignInPolicyWriter_NilCore503Filed pins the fail-closed origin
// on the native writer path: with no core origin configured the
// family route answers the permanent 503 backend_unavailable in both
// negotiated shapes and both spellings, and files the local_503
// outcome on the native lane.
func TestSignInPolicyWriter_NilCore503Filed(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(nil)
	srv := NewServer("", skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		for _, target := range []string{
			"/api/v0/auth/admin/sign-in-policy",
			"/api/v0/auth/admin/sign-in-policy/",
		} {
			req := authedRequest(http.MethodPatch, target)
			if accept != "" {
				req.Header.Set("Accept", accept)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("Accept %q PATCH %s: status = %d, want 503", accept, target, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("Accept %q PATCH %s: Retry-After = %q, want absent (permanent 503)", accept, target, got)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), "backend_unavailable") {
				t.Errorf("Accept %q PATCH %s: body = %q, want backend_unavailable", accept, target, body)
			}
		}
	}
	snap := tel.snapshot()
	if got := snap.dispatches["sign_in_policy"][viaNative][outcomeLocal503]; got != 4 {
		t.Errorf("family sign_in_policy native local_503 = %d, want 4 (two shapes times two spellings)", got)
	}
}

// TestSignInPolicyWriter_DenyBranchTelemetry pins the deny shape for
// the day dest gains a scoped/session resolver: the unreachable deny
// branch files the authorizer deny judgment and the denied dispatch
// outcome on the native lane, and the core is never reached.
func TestSignInPolicyWriter_DenyBranchTelemetry(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(nil)
	srv := NewServer("http://core.invalid", skeletonTestKey).WithTelemetry(tel)

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
	snap := tel.snapshot()
	if got := snap.authorizer["sign_in_policy"][decisionDeny]; got != 2 {
		t.Errorf("authorizer[sign_in_policy][deny] = %d, want 2 (deny judgment stays observable)", got)
	}
	if got := snap.dispatches["sign_in_policy"][viaNative][outcomeDenied]; got != 2 {
		t.Errorf("family sign_in_policy native denied = %d, want 2 (one per deny, no dispatch)", got)
	}
}
