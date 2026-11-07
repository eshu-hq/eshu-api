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

// D9 OIDC-login tier-C dispatch clones (Refs eshu-hq/eshu#9669, part
// of #6707; first of the three sequential family PRs: OIDC, then
// GitHub, then SAML). TDD record: this suite failed RED before the
// implementation landed (frozen route set asserted first, referencing
// the undefined d9Routes table and d9PublicWriteThrough handler) and
// is green with it. Until the lane lands, the routes cross the generic
// byte-relay inside RequireAuth, so the ahead-of-relay assertions fail
// first. That red is the recorded expectation, not a defect.
//
// Scope honesty: these are tier-C dispatch clones — HTTP write-through
// to the current core handlers — not termination and not removal
// readiness. The D9 pin (OIDCLoginAdmission v0 at eshu 40e88be1) is
// the recorded precondition; the dest-side state store, replay cache,
// session mint, and finalize/audit stack stay termination obligations
// for a later lane. GitHub rides its own landed lane (d9GitHubRoutes);
// SAML routes stay relay-only here.

// TestD9RoutesFrozen pins the exact D9 OIDC surface the satellite
// clones. A route added, removed, or renamed fails here until the pin
// contract, the spec fragment, and this mirror move together. GitHub
// and SAML routes must never appear here: SAML rides its own
// sequential lane, and GitHub rides its landed lane table
// (d9GitHubRoutes).
func TestD9RoutesFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/auth/oidc/login",
		"GET /api/v0/auth/oidc/callback",
	}
	got := append([]string(nil), d9Routes...)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("d9Routes holds %d entries %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("d9Routes differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestD9Native_RegisteredAheadOfRelay pins the registration this lane
// exists for: the 2 OIDC GETs cross explicit native routes ahead of
// the generic relay, so the dispatch signal files via="native" under
// the oidc_login family even for keyless callers (the carve-out serves
// them: no dest 401). A route that fell through to the generic proxy
// would file via="relay" instead, and this test would fail:
// byte-identity between the lanes is the point, so only the dispatch
// signal can tell them apart.
func TestD9Native_RegisteredAheadOfRelay(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(discardLogger())
	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://idp.example.com/authorize?state=x"}}, body: ``}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	for _, route := range d9Routes {
		method, path, _ := strings.Cut(route, " ")
		req := httptest.NewRequest(method, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("%s: status = %d, want relayed 302", route, rec.Code)
		}
	}
	if core.hits != len(d9Routes) {
		t.Fatalf("upstream hits = %d, want %d (one write-through per route)", core.hits, len(d9Routes))
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Result().Body)
	text := string(body)
	if want := `eshu_api_dispatch_total{family="oidc_login",via="native",outcome="core_ok"} 2`; !strings.Contains(text, want) {
		t.Errorf("/metrics missing %q\nfull body:\n%s", want, text)
	}
}

// d9ParityOutcomes is the shared outcome matrix the native-vs-relay
// parity proof drives: the stub answers the pinned wire strings where
// core owns the text, and opaque bytes where the relay (not the text)
// is what the test pins. The redirect rows prove dest never follows a
// core redirect: every row below must show exactly one upstream hit on
// the native lane. The 500/503 rows prove dest never retries an
// unstable core: a retried login start would create a second
// server-side state, so one forward is the whole census-mutation
// contract.
var d9ParityOutcomes = []struct {
	name   string
	status int
	header http.Header
	body   string
}{
	{"start 302", http.StatusFound, http.Header{"Location": []string{"https://idp.example.com/authorize?state=opaque&nonce=opaque"}}, ``},
	{"callback 201", http.StatusCreated, http.Header{"Set-Cookie": []string{"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict", "__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict"}}, `{"csrf_token":"opaque-csrf"}`},
	{"callback 303", http.StatusSeeOther, http.Header{"Location": []string{"/console"}, "Set-Cookie": []string{"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict", "__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict"}}, ``},
	{"unconfigured 503", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"oidc login is unavailable"}`},
	{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid oidc login request"}`},
	{"callback 401", http.StatusUnauthorized, nil, `{"error":"Unauthorized","detail":"authentication is required"}`},
	{"denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"oidc login denied"}`},
	{"core 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"oidc login failed"}`},
	{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
	{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the session store is not configured"}`},
}

// d9ParityQuery returns the representative caller query for an OIDC
// route: start carries the provider IDs plus return_to, callback
// carries the credential state + code. Values must cross opaque and
// byte-identical: dest parses, logs, and telemeters none of them.
func d9ParityQuery(path string) string {
	if strings.HasSuffix(path, "/oidc/callback") {
		return "state=st-opaque-123&code=cd-opaque-456"
	}
	return "provider_config_id=okta-dev&tenant_id=tenant_a&workspace_id=workspace_a&return_to=%2Fconsole"
}

// TestD9Native_MatchesRelayByteForByte pins that the tier-C clone
// relays core bytes exactly like the legacy generic proxy it replaces:
// same status, same headers (modulo per-hop framing), same body —
// across the pinned outcome matrix in both negotiated shapes,
// including the redirect status with its Location, the multiple
// Set-Cookie values, and the 503/400/403 error envelopes. The legacy
// lane is replayed by mounting the generic dispatch for the same path
// in a test mux, so the comparison is native-vs-relay on the same
// binary and stub core, not a copy of the implementation.
func TestD9Native_MatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	for _, route := range d9Routes {
		method, path, _ := strings.Cut(route, " ")
		query := d9ParityQuery(path)
		for _, outcome := range d9ParityOutcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				core := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)

				nativeSrv := NewServer(upstream.URL, skeletonTestKey)
				nativeHandler := nativeSrv.Handler(DefaultV0Sunset)
				nativeReq := httptest.NewRequest(method, path+"?"+query, nil)
				nativeReq.Header.Set("Accept", accept)
				nativeRec := httptest.NewRecorder()
				nativeHandler.ServeHTTP(nativeRec, nativeReq)
				nativeResult := nativeRec.Result()
				nativeBody, _ := io.ReadAll(nativeResult.Body)
				upstream.Close()

				if core.hits != 1 {
					t.Errorf("%s %s Accept %q: native upstream hits = %d, want exactly 1 (no retry on write-through)",
						route, outcome.name, accept, core.hits)
				}
				if core.lastQuery != query {
					t.Errorf("%s %s Accept %q: upstream query = %q, want the opaque caller query %q",
						route, outcome.name, accept, core.lastQuery, query)
				}

				relayCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
				relayMux := http.NewServeMux()
				relayMux.Handle("/", relayCore.handler())
				relayUpstream := httptest.NewServer(relayMux)
				defer relayUpstream.Close()

				relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
				legacy := relaySrv.instrumented(viaRelay, RequireAuth(relaySrv.apiKey, http.HandlerFunc(relaySrv.dispatch)))
				relayReq := authedRequest(method, path+"?"+query)
				relayReq.Header.Set("Accept", accept)
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
				for _, key := range []string{"Location", "Retry-After", "Content-Type"} {
					if got, want := nativeResult.Header.Values(key), relayResult.Header.Values(key); !equalHeaders(got, want) {
						t.Errorf("%s %s Accept %q: native %s = %q, relay %s = %q, want equal",
							route, outcome.name, accept, key, got, key, want)
					}
				}
				if got, want := nativeResult.Header.Values("Set-Cookie"), relayResult.Header.Values("Set-Cookie"); !equalHeaders(got, want) {
					t.Errorf("%s %s Accept %q: native Set-Cookie = %q, relay Set-Cookie = %q, want every cookie relayed",
						route, outcome.name, accept, got, want)
				}
			}
		}
	}
}

// equalHeaders reports whether two multi-value header readings carry
// the same values in order.
func equalHeaders(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
