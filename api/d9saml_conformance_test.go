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

// D9 SAML-login tier-C dispatch clones (Refs eshu-hq/eshu#9669, part
// of #6707; third of the three sequential family PRs: OIDC, then
// GitHub, then SAML). TDD record: this suite failed RED before the
// implementation landed (frozen route set asserted first, referencing
// the undefined d9SAMLRoutes table and d9SAMLPublicWriteThrough
// handler) and is green with it. Until the lane lands, the routes
// cross the generic byte-relay inside RequireAuth, so the ahead-of-relay
// assertions fail first. That red is the recorded expectation, not a
// defect.
//
// Scope honesty: these are tier-C dispatch clones — HTTP write-through
// to the current core handlers — not termination and not removal
// readiness. The D9 SAML pin (SAMLLoginAdmissionContractVersion v0 at
// eshu 8c23246933, byte-identical re-anchor of the stale aac271ad pin)
// is the recorded precondition; the dest-side provider material,
// session mint, replay reservation, and finalize/audit stack stay
// termination obligations for a later lane. OIDC and GitHub stay on
// their own landed lanes.

// TestD9SAMLRoutesFrozen pins the exact D9 SAML surface the satellite
// clones. A route added, removed, or renamed fails here until the pin
// contract, the spec fragment, and this mirror move together. OIDC and
// GitHub routes stay on their own lane tables; no other family's route
// may ride this one.
func TestD9SAMLRoutesFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/auth/saml/providers/{provider_id}/metadata",
		"GET /api/v0/auth/saml/providers/{provider_id}/login",
		"POST /api/v0/auth/saml/providers/{provider_id}/acs",
	}
	got := append([]string(nil), d9SAMLRoutes...)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("d9SAMLRoutes holds %d entries %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("d9SAMLRoutes differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestD9SAMLNative_HandlerBuildsPanicFree pins the mux-collision
// contract at build time: the full production Handler (all landed
// lanes plus this one) constructs without a pattern conflict panic,
// and every lane route answers. A pattern that collides with an OIDC,
// GitHub, or relay key panics here before any row below runs.
func TestD9SAMLNative_HandlerBuildsPanicFree(t *testing.T) {
	t.Parallel()

	var handler http.Handler
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("Handler() panicked with %v, want panic-free mux build", recovered)
			}
		}()
		core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
		upstream := httptest.NewServer(core.handler())
		defer upstream.Close()
		srv := NewServer(upstream.URL, skeletonTestKey)
		handler = srv.Handler(DefaultV0Sunset)
	}()
	if handler == nil {
		t.Fatal("Handler() returned nil, want the production handler")
	}
}

// TestD9SAMLNative_RegisteredAheadOfRelay pins the registration this
// lane exists for: the 3 SAML routes cross explicit native routes ahead
// of the generic relay, so the dispatch signal files via="native" under
// the saml_login family even for keyless callers (the carve-out serves
// them: no dest 401). A route that fell through to the generic proxy
// would file via="relay" instead, and this test would fail:
// byte-identity between the lanes is the point, so only the dispatch
// signal can tell them apart.
func TestD9SAMLNative_RegisteredAheadOfRelay(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(discardLogger())
	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	targets := []string{
		"/api/v0/auth/saml/providers/p1/metadata",
		"/api/v0/auth/saml/providers/p1/login",
	}
	for _, target := range targets {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want relayed 200", target, rec.Code)
		}
	}
	acsBody := strings.NewReader("RelayState=opaque-relay& SAMLResponse=opaque-response")
	req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/p1/acs", acsBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST acs: status = %d, want relayed 200", rec.Code)
	}
	if core.hits != len(targets)+1 {
		t.Fatalf("upstream hits = %d, want %d (one write-through per route)", core.hits, len(targets)+1)
	}

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Result().Body)
	text := string(body)
	if want := `eshu_api_dispatch_total{family="saml_login",via="native",outcome="core_ok"} 3`; !strings.Contains(text, want) {
		t.Errorf("/metrics missing %q\nfull body:\n%s", text, want)
	}
}

// d9SAMLParityOutcomes is the shared outcome matrix the
// native-vs-relay parity proof drives: the stub answers the pinned wire
// strings where core owns the text, and opaque bytes where the relay
// (not the text) is what the test pins. The redirect rows prove dest
// never follows a core redirect: every row below must show exactly one
// upstream hit on the native lane. The 500/503 rows prove dest never
// retries an unstable core: a retried ACS POST would spend the
// AuthnRequest core-side (a commit spends it), so one forward is the
// whole exactly-once story dest-side. The 401 row pins the
// malformed-response envelope and the 404 row pins the unknown-provider
// answer: dest synthesizes neither.
var d9SAMLParityOutcomes = []struct {
	name   string
	status int
	header http.Header
	body   string
}{
	{"metadata 200", http.StatusOK, http.Header{"Content-Type": []string{"application/samlmetadata+xml"}}, `<EntityDescriptor entityID="https://eshu.example/saml/p1"/>`},
	{"login 302", http.StatusFound, http.Header{"Location": []string{"https://idp.example.com/sso?SAMLRequest=opaque&RelayState=opaque"}}, ``},
	{"login duplicate 302", http.StatusFound, http.Header{"Location": []string{"https://idp.example.com/sso?SAMLRequest=opaque-2&RelayState=opaque-2"}}, ``},
	{"login refused 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"saml provider not found"}`},
	{"acs 201", http.StatusCreated, http.Header{"Set-Cookie": []string{"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict", "__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict"}}, `{"csrf_token":"opaque-csrf"}`},
	{"acs duplicate 401", http.StatusUnauthorized, nil, `{"error":"Unauthorized","detail":"authentication is required"}`},
	{"acs 303", http.StatusSeeOther, http.Header{"Location": []string{"/console"}, "Set-Cookie": []string{"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict", "__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict"}}, ``},
	{"acs refused 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"RelayState and SAMLResponse are required"}`},
	{"core 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"saml login failed"}`},
	{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
	{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the session store is not configured"}`},
}

// d9SAMLParityTarget returns the representative caller request for a
// SAML route: metadata and login carry no caller body (login carries
// the optional same-origin return_to query), ACS carries the opaque
// IdP-posted form bytes. Values must cross opaque and byte-identical:
// dest parses, logs, and telemeters none of them.
func d9SAMLParityTarget(path string) (method, query, body string) {
	if strings.HasSuffix(path, "/acs") {
		return http.MethodPost, "", "RelayState=opaque-relay& SAMLResponse=opaque-response"
	}
	if strings.HasSuffix(path, "/login") {
		return http.MethodGet, "return_to=%2Fconsole", ""
	}
	return http.MethodGet, "", ""
}

// TestD9SAMLNative_MatchesRelayByteForByte pins that the tier-C clone
// relays core bytes exactly like the legacy generic proxy it replaces:
// same status, same headers (modulo per-hop framing), same body —
// across the pinned outcome matrix in both negotiated shapes,
// including the metadata XML content type, the redirect status with
// its Location, the multiple Set-Cookie values, and the 400/401/404
// error envelopes. The legacy lane is replayed by mounting the generic
// dispatch for the same path in a test mux, so the comparison is
// native-vs-relay on the same binary and stub core, not a copy of the
// implementation.
func TestD9SAMLNative_MatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	for _, route := range d9SAMLRoutes {
		_, path, _ := strings.Cut(route, " ")
		method, query, form := d9SAMLParityTarget(path)
		for _, outcome := range d9SAMLParityOutcomes {
			// Metadata rows only run against the metadata route, ACS
			// rows only against ACS, login rows against login: the
			// matrix names its route.
			if strings.HasPrefix(outcome.name, "metadata") && !strings.HasSuffix(path, "/metadata") {
				continue
			}
			if strings.HasPrefix(outcome.name, "login") && !strings.HasSuffix(path, "/login") {
				continue
			}
			if strings.HasPrefix(outcome.name, "acs") && !strings.HasSuffix(path, "/acs") {
				continue
			}
			if strings.HasPrefix(outcome.name, "core ") && strings.HasSuffix(path, "/metadata") {
				continue
			}
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				core := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)

				nativeSrv := NewServer(upstream.URL, skeletonTestKey)
				nativeHandler := nativeSrv.Handler(DefaultV0Sunset)
				target := path
				if query != "" {
					target += "?" + query
				}
				var nativeReq *http.Request
				if form != "" {
					nativeReq = httptest.NewRequest(method, target, strings.NewReader(form))
					nativeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				} else {
					nativeReq = httptest.NewRequest(method, target, nil)
				}
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
				if form != "" && core.lastBody != form {
					t.Errorf("%s %s Accept %q: upstream body = %q, want the opaque caller form %q",
						route, outcome.name, accept, core.lastBody, form)
				}

				relayCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
				relayMux := http.NewServeMux()
				relayMux.Handle("/", relayCore.handler())
				relayUpstream := httptest.NewServer(relayMux)
				defer relayUpstream.Close()

				relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
				legacy := relaySrv.instrumented(viaRelay, RequireAuth(relaySrv.apiKey, http.HandlerFunc(relaySrv.dispatch)))
				var relayReq *http.Request
				if form != "" {
					relayReq = httptest.NewRequest(method, target, strings.NewReader(form))
					relayReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				} else {
					relayReq = httptest.NewRequest(method, target, nil)
				}
				relayReq.Header.Set("Authorization", "Bearer "+skeletonTestKey)
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
