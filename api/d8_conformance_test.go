// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// D8 session-resolver tier-C dispatch clones (Refs eshu-hq/eshu#9669,
// part of #6707). TDD record: this suite failed RED before the
// implementation landed (frozen route set asserted first, referencing
// the undefined d8Routes table and d8WriteThroughAs handler) and is
// green with it. Until the lane lands, the routes cross the generic
// byte-relay, so the ahead-of-relay assertions fail first. That red is
// the recorded expectation, not a defect.
//
// Scope honesty: these are tier-C dispatch clones — HTTP write-through
// to the current core handlers — not termination and not removal
// readiness. The D8 pins (SessionResolver, BrowserSession,
// LocalIdentity, APIToken v0) are the recorded preconditions; the
// dest-side resolver, ledger, client-stable keys, and finalize/audit
// stack stay termination obligations for the later lane.

// TestD8RoutesFrozen pins the exact D8 surface the satellite clones.
// A route added, removed, or renamed fails here until the pin
// contract, the spec fragment, and this mirror move together.
func TestD8RoutesFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/auth/browser-session",
		"GET /api/v0/auth/browser-session",
		"DELETE /api/v0/auth/browser-session",
		"PATCH /api/v0/auth/browser-session/context",
		"GET /api/v0/auth/sessions",
		"GET /api/v0/auth/profile",
		"POST /api/v0/auth/local/bootstrap",
		"POST /api/v0/auth/local/login",
		"POST /api/v0/auth/local/invitations",
		"POST /api/v0/auth/local/invitations/accept",
		"POST /api/v0/auth/local/users/{user_id}/password",
		"POST /api/v0/auth/local/password/rotate",
		"POST /api/v0/auth/local/users/{user_id}/mfa-reset",
		"POST /api/v0/auth/local/users/{user_id}/disable",
		"POST /api/v0/auth/local/break-glass",
		"POST /api/v0/auth/local/break-glass/session",
		"GET /api/v0/auth/local/api-tokens",
		"POST /api/v0/auth/local/api-tokens",
		"POST /api/v0/auth/local/api-tokens/{token_id}/revoke",
		"POST /api/v0/auth/local/api-tokens/{token_id}/rotate",
		"POST /api/v0/auth/local/mfa/totp/begin",
		"POST /api/v0/auth/local/mfa/totp/confirm",
	}
	got := append([]string(nil), d8Routes...)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("d8Routes holds %d entries %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("d8Routes differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestD8GatedAndPublicSplit pins the lane's two halves: the 18 gated
// routes cross the credential-class gate and the 4 public carve-out
// routes stay outside every gate. A route on the wrong half fails here.
func TestD8GatedAndPublicSplit(t *testing.T) {
	t.Parallel()

	if len(d8GatedRoutes) != 18 {
		t.Fatalf("d8GatedRoutes holds %d entries, want the 18 authed D8 routes", len(d8GatedRoutes))
	}
	if len(d8PublicRoutes) != 4 {
		t.Fatalf("d8PublicRoutes holds %d entries, want the 4 pre-auth public routes", len(d8PublicRoutes))
	}
	seen := map[string]string{}
	for _, route := range d8GatedRoutes {
		seen[route] = "gated"
	}
	for _, route := range d8PublicRoutes {
		if prev, dup := seen[route]; dup {
			t.Errorf("route %s is both %s and public, want exactly one half", route, prev)
		}
		seen[route] = "public"
	}
	for _, route := range []string{
		"POST /api/v0/auth/local/login",
		"POST /api/v0/auth/local/invitations/accept",
		"POST /api/v0/auth/local/break-glass/session",
		"POST /api/v0/auth/local/password/rotate",
	} {
		if seen[route] != "public" {
			t.Errorf("route %s rides %q, want the public carve-out", route, seen[route])
		}
	}
	if seen["POST /api/v0/auth/local/bootstrap"] != "gated" {
		t.Errorf("bootstrap rides %q, want the gated half (shared-operator gate preserved)",
			seen["POST /api/v0/auth/local/bootstrap"])
	}
}

// TestD8Native_RegisteredAheadOfRelay pins the registration this lane
// exists for: the 22 D8 routes cross explicit native routes ahead of
// the generic relay, so the dispatch signal files via="native" under
// each route's family. A route that fell through to the generic proxy
// would file via="relay" instead, and this test would fail:
// byte-identity between the lanes is the point, so only the dispatch
// signal can tell them apart.
//
// Family note: the invitations create POST and the invitations-accept
// carve-out file under the pre-existing admin_identity telemetry
// prefix (longest-prefix match on /auth/local/invitations); every other
// LocalIdentity route files local_identity. The contract discloses the
// split: telemetry family is the closed metrics label, not the
// admission family.
func TestD8Native_RegisteredAheadOfRelay(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(discardLogger())
	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	for _, route := range d8Routes {
		method, path, _ := strings.Cut(route, " ")
		req := authedRequest(method, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want relayed 200", route, rec.Code)
		}
	}
	if core.hits != len(d8Routes) {
		t.Fatalf("upstream hits = %d, want %d (one write-through per route)", core.hits, len(d8Routes))
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Result().Body)
	text := string(body)
	for _, want := range []string{
		`eshu_api_dispatch_total{family="browser_session",via="native",outcome="core_ok"} 4`,
		`eshu_api_dispatch_total{family="session_list",via="native",outcome="core_ok"} 1`,
		`eshu_api_dispatch_total{family="profile",via="native",outcome="core_ok"} 1`,
		`eshu_api_dispatch_total{family="admin_identity",via="native",outcome="core_ok"} 2`,
		`eshu_api_dispatch_total{family="local_identity",via="native",outcome="core_ok"} 14`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("/metrics missing %q\nfull body:\n%s", want, text)
		}
	}
}

// d8ParityOutcomes is the shared outcome matrix the native-vs-relay
// parity proof drives: the stub answers the pinned wire strings where
// core owns the text, and opaque bytes where the relay (not the text)
// is what the test pins. The 500/503 rows prove the dest never retries
// an unstable core: every row below must show exactly one upstream hit
// on the native lane.
var d8ParityOutcomes = []struct {
	name   string
	status int
	header http.Header
	body   string
}{
	{"accepted 200", http.StatusOK, nil, `{"status":"accepted"}`},
	{"created 201", http.StatusCreated, nil, `{"status":"created"}`},
	{"revoked 204", http.StatusNoContent, nil, ``},
	{"mfa required 202", http.StatusAccepted, nil, `{"status":"mfa_required"}`},
	{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"current password is required"}`},
	{"auth failed 401", http.StatusUnauthorized, nil, `{"error":"Unauthorized","detail":"local identity authentication failed"}`},
	{"core 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scopes operator authorization is required"}`},
	{"core 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"unknown token"}`},
	{"core 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"store is unavailable"}`},
	{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
	{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the session store is not configured"}`},
}

// d8ParityPayload returns the representative caller bytes for a D8
// route: credential-bearing POST bodies carry realistic proofs (they
// must cross opaque and byte-identical), reads carry no body.
func d8ParityPayload(method, path string) string {
	if method == http.MethodGet || method == http.MethodDelete {
		return ""
	}
	switch {
	case strings.HasSuffix(path, "/local/login"):
		return `{"username":"operator","password":"s3cret-proof","totp_code":"123456"}`
	case strings.HasSuffix(path, "/password/rotate"):
		return `{"username":"operator","current_password":"old-proof","new_password":"new-proof","totp_code":"123456"}`
	case strings.HasSuffix(path, "/invitations/accept"):
		return `{"invite_code":"opaque-proof","username":"operator","password":"s3cret-proof"}`
	case strings.HasSuffix(path, "/break-glass/session"):
		return `{"break_glass_code":"opaque-proof","username":"operator","password":"s3cret-proof"}`
	case strings.HasSuffix(path, "/local/bootstrap"):
		return `{"username":"owner","password":"first-proof"}`
	case strings.HasSuffix(path, "/invitations"):
		return `{"role_id":"r1","tenant_id":"t1","workspace_id":"w1"}`
	case strings.Contains(path, "/password"):
		return `{"new_password":"rotated-proof","current_password":"old-proof"}`
	case strings.HasSuffix(path, "/api-tokens"):
		return `{"token_class":"personal","display_label":"ci"}`
	case strings.Contains(path, "/api-tokens/"):
		return `{}`
	case strings.HasSuffix(path, "/totp/begin"):
		return `{"current_password":"s3cret-proof"}`
	case strings.HasSuffix(path, "/totp/confirm"):
		return `{"code":"123456"}`
	case strings.HasSuffix(path, "/break-glass"):
		return `{"break_glass_code":"envelope-proof","ttl":"15m"}`
	case strings.Contains(path, "/mfa-reset") || strings.Contains(path, "/disable"):
		return `{"note":"operator action"}`
	case strings.HasSuffix(path, "/browser-session"):
		return `{"ttl":"8h"}`
	case strings.HasSuffix(path, "/browser-session/context"):
		return `{"tenant_id":"t1","workspace_id":"w1"}`
	default:
		return `{}`
	}
}

// TestD8Native_MatchesRelayByteForByte pins that the tier-C clone
// relays core bytes exactly like the legacy generic proxy it replaces:
// same status, same headers (modulo per-hop framing), same body —
// across the pinned outcome matrix in both negotiated shapes. The
// legacy lane is replayed by mounting the generic dispatch for the
// same path in a test mux, so the comparison is native-vs-relay on the
// same binary and stub core, not a copy of the implementation.
func TestD8Native_MatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	for _, route := range d8Routes {
		method, path, _ := strings.Cut(route, " ")
		payload := d8ParityPayload(method, path)
		for _, outcome := range d8ParityOutcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				core := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)

				nativeSrv := NewServer(upstream.URL, skeletonTestKey)
				nativeHandler := nativeSrv.Handler(DefaultV0Sunset)
				nativeReq := authedRequest(method, path)
				nativeReq.Header.Set("Accept", accept)
				if payload != "" {
					nativeReq.Body = io.NopCloser(strings.NewReader(payload))
				}
				nativeRec := httptest.NewRecorder()
				nativeHandler.ServeHTTP(nativeRec, nativeReq)
				nativeResult := nativeRec.Result()
				nativeBody, _ := io.ReadAll(nativeResult.Body)
				upstream.Close()

				if core.hits != 1 {
					t.Errorf("%s %s Accept %q: native upstream hits = %d, want exactly 1 (no retry on write-through)",
						route, outcome.name, accept, core.hits)
				}

				relayCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
				relayMux := http.NewServeMux()
				relayMux.Handle("/", relayCore.handler())
				relayUpstream := httptest.NewServer(relayMux)
				defer relayUpstream.Close()

				relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
				legacy := relaySrv.instrumented(viaRelay, RequireAuth(relaySrv.apiKey, http.HandlerFunc(relaySrv.dispatch)))
				relayReq := authedRequest(method, path)
				relayReq.Header.Set("Accept", accept)
				if payload != "" {
					relayReq.Body = io.NopCloser(strings.NewReader(payload))
				}
				relayRec := httptest.NewRecorder()
				legacy.ServeHTTP(relayRec, relayReq)
				relayResult := relayRec.Result()
				relayBody, _ := io.ReadAll(relayResult.Body)

				if nativeRec.Code != relayRec.Code {
					t.Errorf("%s %s Accept %q: native status = %d, relay status = %d, want equal",
						route, outcome.name, accept, nativeRec.Code, relayRec.Code)
				}
				if string(nativeBody) != string(relayBody) {
					t.Errorf("%s %s Accept %q: native body = %q, relay body = %q, want byte-identical",
						route, outcome.name, accept, nativeBody, relayBody)
				}
				for _, key := range []string{"Retry-After", "Content-Type"} {
					if got, want := nativeResult.Header.Get(key), relayResult.Header.Get(key); got != want {
						t.Errorf("%s %s Accept %q: native %s = %q, relay %s = %q, want equal",
							route, outcome.name, accept, key, got, key, want)
					}
				}
			}
		}
	}
}
