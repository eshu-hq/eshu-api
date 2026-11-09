// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-api/openapi"
)

// SAML-triple termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72). The D0 proof per group: dest calls
// the frozen SAMLLoginAdmission v0 port (three methods, exact frozen
// signatures) over D0-proven transport and takes no table or store
// handle; the provider path value, the return_to query value, and the
// ACS form fields cross as opaque bytes; the D8 resolver stays
// complete downstream (session issuance keeps its BrowserSession
// shape); the served spec carries all three ops line-verbatim.
// TDD RED: this file names the port that does not exist yet, so the
// package must fail to compile before the lane lands.

// frozenSAMLTermPort pins the exact dest-side method set behind the
// versioned #9669 admission contract for the saml triple: the three
// admission calls the terminated metadata GET, login GET, and ACS
// POST admit through. The metadata leg admits through GetSAMLProvider
// core-side, but dest never names that method: it returns the SP key
// material (service-provider private key, IdP metadata) that must
// never leave core, so the dest-side metadata call is FetchSAMLMetadata
// — same opaque transport shape, deliberately different name, and the
// check-parity.sh negative grep fails any GetSAMLProvider sighting in
// the lane file. The login leg admits through CreateSAMLRequest
// (named StartSAMLLogin dest-side, mirroring the OIDC/GitHub Start
// convention) and the ACS leg admits through the
// ConsumeSAMLRequest + ReserveSAMLReplay + ResolveSAMLPrincipal chain
// (named CompleteSAMLLogin dest-side). The names stay dest-owned and
// opaque (no core import) because dest transports caller bytes, it
// does not re-implement admission — so a re-signed core method fails
// the frozen test the same way. Bump SAMLLoginAdmissionContractVersion
// alongside (manually — the bump is convention, the frozen list is the
// enforcement).
var frozenSAMLTermPort = []string{
	"CompleteSAMLLogin func(context.Context, api.samlPortRequest) (api.samlPortResponse, error)",
	"FetchSAMLMetadata func(context.Context, api.samlPortRequest) (api.samlPortResponse, error)",
	"StartSAMLLogin func(context.Context, api.samlPortRequest) (api.samlPortResponse, error)",
}

// samlTermPortMethodSet reflects the live samlLoginPort method set
// into sorted "name signature" strings for comparison with the frozen
// list.
func samlTermPortMethodSet(t *testing.T) []string {
	t.Helper()
	rt := reflect.TypeOf((*samlLoginPort)(nil)).Elem()
	var got []string
	for i := 0; i < rt.NumMethod(); i++ {
		m := rt.Method(i)
		got = append(got, m.Name+" "+m.Type.String())
	}
	sort.Strings(got)
	return got
}

// TestSAMLTermPortIsFrozen pins the D0 port: the dest-side seam is
// exactly the three frozen admission calls. The version constant stays
// v0 (api/version.go, pinned against
// contracts/saml-login-admission.md by version_test.go); the frozen
// method set below is the enforcement.
func TestSAMLTermPortIsFrozen(t *testing.T) {
	t.Parallel()

	got := samlTermPortMethodSet(t)
	if len(got) != len(frozenSAMLTermPort) {
		t.Fatalf("samlLoginPort method set = %q (len %d), want frozen %q (len %d)",
			got, len(got), frozenSAMLTermPort, len(frozenSAMLTermPort))
	}
	for i := range got {
		if got[i] != frozenSAMLTermPort[i] {
			t.Errorf("samlLoginPort differs at index %d: got %q, want %q", i, got[i], frozenSAMLTermPort[i])
		}
	}
	if SAMLLoginAdmissionContractVersion != "v0" {
		t.Errorf("SAMLLoginAdmissionContractVersion = %q, want %q: bump it deliberately with the shape change",
			SAMLLoginAdmissionContractVersion, "v0")
	}
}

// TestSAMLTermGroup_SpecCarriesTripleOps pins the OpenAPI assembly:
// the served document carries all three terminated SAML ops with the
// OIDC and GitHub ops alongside. Fragments stay line-verbatim under
// scripts/check-parity.sh; this test pins the assembly around them.
func TestSAMLTermGroup_SpecCarriesTripleOps(t *testing.T) {
	t.Parallel()

	doc := openapi.Spec()
	for _, want := range []string{
		`"/api/v0/auth/saml/providers/{provider_id}/metadata"`,
		`"/api/v0/auth/saml/providers/{provider_id}/login"`,
		`"/api/v0/auth/saml/providers/{provider_id}/acs"`,
		`"/api/v0/auth/oidc/login"`,
		`"/api/v0/auth/github/login"`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("served spec lacks %s", want)
		}
	}
}

// TestSAMLTermPathsMirrorD9SAMLRoutes pins that the terminated triple
// is exactly the frozen D9 SAML table: a route added, removed, or
// renamed here fails until the D9 table, the OpenAPI fragment, and
// this mirror move together. Only GET spellings terminate on the
// metadata/login templates and only the POST spelling on the ACS
// template.
func TestSAMLTermPathsMirrorD9SAMLRoutes(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		d9SAMLMetadata: "/api/v0/auth/saml/providers/{provider_id}/metadata",
		d9SAMLLogin:    "/api/v0/auth/saml/providers/{provider_id}/login",
		d9SAMLACS:      "/api/v0/auth/saml/providers/{provider_id}/acs",
	}
	for route, path := range want {
		method, routePath, _ := strings.Cut(route, " ")
		if routePath != path {
			t.Errorf("route %q: path = %q, want terminated %q", route, routePath, path)
		}
		switch path {
		case "/api/v0/auth/saml/providers/{provider_id}/acs":
			if method != http.MethodPost {
				t.Errorf("route %q: method = %q, want POST (only the ACS POST terminates here)", route, method)
			}
		default:
			if method != http.MethodGet {
				t.Errorf("route %q: method = %q, want GET (only GETs terminate here)", route, method)
			}
		}
	}
}

// samlTermParityOutcomes is the shared outcome matrix the
// native-vs-relay parity proof drives: the stub answers the pinned
// wire strings where core owns the text, and opaque bytes where the
// relay (not the text) is what the test pins. The redirect rows prove
// dest never follows a core redirect: every row below must show
// exactly one upstream hit on the native lane. The 500/503 rows prove
// dest never retries an unstable core: a retried ACS POST would spend
// the AuthnRequest twice core-side, so one forward is the whole
// exactly-once story dest-side. The 401 row pins the spent-request
// envelope and the 404 rows pin the unknown-provider answer: dest
// synthesizes neither.
var samlTermParityOutcomes = []struct {
	name   string
	status int
	header http.Header
	body   string
}{
	{"metadata 200", http.StatusOK, http.Header{"Content-Type": []string{"application/samlmetadata+xml"}}, `<EntityDescriptor entityID="https://eshu.example/saml/acme-munich"/>`},
	{"metadata unknown-provider 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"saml provider not found"}`},
	{"login 302", http.StatusFound, http.Header{"Location": []string{"https://idp.example.com/sso?SAMLRequest=opaque&RelayState=opaque&SigAlg=opaque-sigalg&Signature=opaque-signature"}}, ``},
	{"login duplicate 302", http.StatusFound, http.Header{"Location": []string{"https://idp.example.com/sso?SAMLRequest=opaque-2&RelayState=opaque-2&SigAlg=opaque-sigalg&Signature=opaque-signature-2"}}, ``},
	{"login refused 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"saml provider not found"}`},
	{"acs 201", http.StatusCreated, http.Header{"Set-Cookie": []string{"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict", "__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict"}}, `{"csrf_token":"opaque-csrf"}`},
	{"acs duplicate 401", http.StatusUnauthorized, nil, `{"error":"Unauthorized","detail":"authentication is required"}`},
	{"acs 303", http.StatusSeeOther, http.Header{"Location": []string{"/console"}, "Set-Cookie": []string{"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict", "__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict"}}, ``},
	{"acs refused 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"RelayState and SAMLResponse are required"}`},
	{"core 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"saml login failed"}`},
	{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
	{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the session store is not configured"}`},
}

// samlTermParityTarget returns the representative caller request for
// a terminated SAML path: metadata carries the provider path value,
// login carries the provider path value plus the optional same-origin
// return_to query, ACS carries the opaque IdP-posted form bytes.
// Values must cross opaque and byte-identical: dest parses, logs, and
// telemeters none of them.
func samlTermParityTarget(path string) (method, query, body string) {
	if strings.HasSuffix(path, "/acs") {
		return http.MethodPost, "", "RelayState=opaque-relay& SAMLResponse=opaque-response"
	}
	if strings.HasSuffix(path, "/login") {
		return http.MethodGet, "return_to=%2Fconsole", ""
	}
	return http.MethodGet, "", ""
}

// samlTermParityPath returns the concrete caller path for a d9SAMLRoutes
// template: the wildcard stands in for one provider segment.
func samlTermParityPath(t *testing.T, route string) string {
	t.Helper()
	_, template, _ := strings.Cut(route, " ")
	path := strings.Replace(template, "{provider_id}", "acme-munich", 1)
	if strings.Contains(path, "{") {
		t.Fatalf("route %q: concrete path %q still holds a template variable", route, path)
	}
	return path
}

// TestSAMLTerm_MatchesRelayByteForByte pins that the terminated triple
// answers exactly like the legacy generic relay it replaces: same
// status, same headers (Location, Retry-After, Content-Type,
// Set-Cookie per key in order), same body — across the pinned outcome
// matrix in both negotiated shapes. The legacy lane is replayed by
// mounting the generic dispatch for the same path in a test mux, so
// the comparison is terminated-vs-relay on the same binary and stub
// core, not a copy of the implementation.
func TestSAMLTerm_MatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	for _, route := range d9SAMLRoutes {
		path := samlTermParityPath(t, route)
		method, query, form := samlTermParityTarget(path)
		for _, outcome := range samlTermParityOutcomes {
			// Metadata rows only run against the metadata path, login
			// rows only against login, ACS rows only against ACS: the
			// matrix names its leg.
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
					t.Errorf("%s %s Accept %q: terminated upstream hits = %d, want exactly 1 (no retry)",
						route, outcome.name, accept, core.hits)
				}
				if core.lastQuery != query {
					t.Errorf("%s %s Accept %q: upstream query = %q, want the opaque caller query %q",
						route, outcome.name, accept, core.lastQuery, query)
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
					t.Errorf("%s %s Accept %q: terminated status = %d, relay status = %d, want equal",
						route, outcome.name, accept, nativeRec.Code, relayRec.Code)
				}
				if string(nativeBody) != string(relayBody) {
					t.Errorf("%s %s Accept %q: terminated body = %q, relay body = %q, want byte-identical",
						route, outcome.name, accept, nativeBody, relayBody)
				}
				for _, key := range []string{"Location", "Retry-After", "Content-Type"} {
					if got, want := nativeResult.Header.Values(key), relayResult.Header.Values(key); !equalHeaders(got, want) {
						t.Errorf("%s %s Accept %q: terminated %s = %q, relay %s = %q, want equal",
							route, outcome.name, accept, key, got, key, want)
					}
				}
				if got, want := nativeResult.Header.Values("Set-Cookie"), relayResult.Header.Values("Set-Cookie"); !equalHeaders(got, want) {
					t.Errorf("%s %s Accept %q: terminated Set-Cookie = %q, relay Set-Cookie = %q, want every cookie verbatim",
						route, outcome.name, accept, got, want)
				}
			}
		}
	}
}

// TestSAMLTerm_RedirectUnfollowed pins the redirect contract: a core
// 302 relays as a 302 with its Location (SigAlg/Signature intact),
// and dest never issues the follow-up fetch. The Location points back
// at the same stub, so a followed redirect would land a second hit;
// exactly one hit plus the relayed 302 proves the shared dispatch
// client holds ErrUseLastResponse on this lane too.
func TestSAMLTerm_RedirectUnfollowed(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusFound,
		header: http.Header{"Location": []string{"/api/v0/auth/saml/providers/acme-munich/acs"}},
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want the core's 302 unfollowed", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/api/v0/auth/saml/providers/acme-munich/acs" {
		t.Errorf("Location = %q, want the core's value byte-verbatim", got)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (no follow-up fetch)", core.hits)
	}
}

// TestSAMLTerm_ACS303CarriesCookiesBeforeRedirect pins the ACS 303
// synthesis: both session cookies land before the redirect to the
// stored same-origin return path, with the Location byte-verbatim.
func TestSAMLTerm_ACS303CarriesCookiesBeforeRedirect(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusSeeOther,
		header: http.Header{
			"Location": []string{"/console"},
			"Set-Cookie": []string{
				"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict",
				"__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict",
			},
		},
		body: ``,
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	form := "RelayState=opaque-relay& SAMLResponse=opaque-response"
	req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want the post-login 303", rec.Code)
	}
	cookies := rec.Header().Values("Set-Cookie")
	if len(cookies) != 2 {
		t.Fatalf("Set-Cookie count = %d, want 2 verbatim (cookies before redirect)", len(cookies))
	}
	if got := rec.Header().Get("Location"); got != "/console" {
		t.Errorf("Location = %q, want the stored return path byte-verbatim", got)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (no retry of ACS mutation)", core.hits)
	}
}

// TestSAMLTerm_NoRetryOnUnstable pins the single-attempt rule: the
// core 500, the retryable 503 (whose Retry-After relays untouched),
// the permanent 503, and core's own form-400 each cost exactly one
// upstream attempt on every leg. A retried login start would create a
// second server-side state; a retried ACS would spend the request
// twice — one forward is the whole exactly-once story dest-side.
func TestSAMLTerm_NoRetryOnUnstable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"core 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"saml login failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
		{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the session store is not configured"}`},
		{"acs refused 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"RelayState and SAMLResponse are required"}`},
	}
	for _, tc := range cases {
		core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		for _, route := range d9SAMLRoutes {
			path := samlTermParityPath(t, route)
			method, query, form := samlTermParityTarget(path)
			target := path
			if query != "" {
				target += "?" + query
			}
			var req *http.Request
			if form != "" {
				req = httptest.NewRequest(method, target, strings.NewReader(form))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest(method, target, nil)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Errorf("%s %s: upstream hits = %d, want exactly 1 (no dest retry)", tc.name, route, core.hits)
			}
			if rec.Code != tc.status {
				t.Errorf("%s %s: status = %d, want core status %d verbatim", tc.name, route, rec.Code, tc.status)
			}
			core.hits = 0
		}
	}
}

// TestSAMLTerm_DeadCoreSingleAttempt pins the dead-core shape: an
// unreachable core answers the permanent local 503
// backend_unavailable with no Retry-After, in both negotiated
// shapes, after exactly one attempt that fails inside the client.
func TestSAMLTerm_DeadCoreSingleAttempt(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	deadURL := upstream.URL
	upstream.Close()

	srv := NewServer(deadURL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, route := range d9SAMLRoutes {
		path := samlTermParityPath(t, route)
		method, _, form := samlTermParityTarget(path)
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			var req *http.Request
			if form != "" {
				req = httptest.NewRequest(method, path, strings.NewReader(form))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest(method, path, nil)
			}
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s Accept %q: status = %d, want permanent local 503", route, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%s Accept %q: Retry-After = %q, want empty (permanent, never retryable)", route, accept, got)
			}
		}
	}
}

// TestSAMLTerm_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: with no core origin configured all three terminated
// spellings answer the permanent 503 backend_unavailable in both
// negotiated shapes, with zero core contact to make.
func TestSAMLTerm_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, route := range d9SAMLRoutes {
		path := samlTermParityPath(t, route)
		method, _, form := samlTermParityTarget(path)
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			var req *http.Request
			if form != "" {
				req = httptest.NewRequest(method, path, strings.NewReader(form))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest(method, path, nil)
			}
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("%s Accept %q: panicked with %v, want 503", route, accept, recovered)
					}
				}()
				handler.ServeHTTP(rec, req)
			}()

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s Accept %q: status = %d, want the permanent 503", route, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%s Accept %q: Retry-After = %q, want empty (permanent, never retryable)", route, accept, got)
			}
		}
	}
}

// TestSAMLTerm_DisabledProviderDenyShape pins the disabled-provider
// answer: a core with no SAML provider configured refuses with its
// 503 unavailable shape, and the terminated metadata leg relays that
// shape verbatim instead of inventing a dest-side refusal or leaking
// provider detail.
func TestSAMLTerm_DisabledProviderDenyShape(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{status: http.StatusServiceUnavailable, body: `{"error":"Service Unavailable","detail":"saml login is unavailable"}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/metadata", nil)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("Accept %q: status = %d, want the disabled-provider 503", accept, rec.Code)
		}
		raw, _ := io.ReadAll(rec.Result().Body)
		if string(raw) != `{"error":"Service Unavailable","detail":"saml login is unavailable"}` {
			t.Errorf("Accept %q: body = %q, want the deny shape verbatim", accept, raw)
		}
		if core.hits != 1 {
			t.Errorf("Accept %q: upstream hits = %d, want exactly 1", accept, core.hits)
		}
	}
}

// TestSAMLTerm_UnknownProviderForwards pins the wildcard
// mux-collision contract for unknown provider IDs: the wildcard
// segment matches any single segment — including one core never
// configured — so dest forwards opaquely (upstream sees the unknown
// ID untouched) and relays core's 404 verbatim instead of inventing
// a dest-side refusal.
func TestSAMLTerm_UnknownProviderForwards(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusNotFound, body: `{"error":"Not Found","detail":"saml provider not found"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range []struct {
		method string
		path   string
		query  string
		body   string
	}{
		{http.MethodGet, "/api/v0/auth/saml/providers/never-configured-zzz/metadata", "", ""},
		{http.MethodGet, "/api/v0/auth/saml/providers/never-configured-zzz/login", "return_to=%2Fconsole", ""},
		{http.MethodPost, "/api/v0/auth/saml/providers/never-configured-zzz/acs", "", "RelayState=opaque-relay& SAMLResponse=opaque-response"},
	} {
		target := tc.path
		if tc.query != "" {
			target += "?" + tc.query
		}
		var req *http.Request
		if tc.body != "" {
			req = httptest.NewRequest(tc.method, target, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(tc.method, target, nil)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want core's unknown-provider 404 verbatim", tc.method, tc.path, rec.Code)
		}
		if core.lastPath != tc.path {
			t.Errorf("%s %s: upstream path = %q, want the unknown ID forwarded untouched", tc.method, tc.path, core.lastPath)
		}
	}
	if core.hits != 3 {
		t.Errorf("upstream hits = %d, want 3 (one forward per unknown-provider leg)", core.hits)
	}
}

// TestSAMLTerm_SiblingPathsStayRelayed pins the other half of the
// wildcard mux-collision contract: multi-segment provider spellings
// and deeper subtrees under a carved path match no wildcard route, so
// they fall through to the keyed generic relay and keep the standing
// 401 for credential-less callers with zero core contact.
func TestSAMLTerm_SiblingPathsStayRelayed(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"GET /api/v0/auth/saml/providers/a/b/login",
		"GET /api/v0/auth/saml/providers/a/b/metadata",
		"GET /api/v0/auth/saml/providers/p1/login/extra",
		"GET /api/v0/auth/saml/providers/p1/metadata/extra",
		"POST /api/v0/auth/saml/providers/p1/acs/extra",
	} {
		method, path, _ := strings.Cut(target, " ")
		req := httptest.NewRequest(method, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want the standing 401 (no wildcard match)", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (sibling paths never dispatch)", core.hits)
	}
}

// TestSAMLTerm_TerminatedFilesNoAuthorizer pins the telemetry
// extension without renames: the terminated SAML legs file their
// family on the existing native lane with the closed outcome
// vocabulary and time the port fetches — and file no authorizer
// judgment, because the routes are public and there is no gate to
// judge.
func TestSAMLTerm_TerminatedFilesNoAuthorizer(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	tel := NewTelemetry(nil)
	srv.WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	targets := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/metadata", ""},
		{http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", ""},
		{http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", "RelayState=opaque& SAMLResponse=opaque"},
	}
	for _, tc := range targets {
		var req *http.Request
		if tc.body != "" {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("%s %s status = %d, want 302", tc.method, tc.path, rec.Code)
		}
	}

	snap := tel.snapshot()
	if snap.dispatches["saml_login"]["native"]["core_ok"] != 3 {
		t.Errorf("dispatch rows = %v, want three saml_login/native/core_ok", snap.dispatches["saml_login"])
	}
	if _, ok := snap.authorizer["saml_login"]; ok {
		t.Errorf("authorizer rows = %v, want none for public saml_login (no gate to judge)", snap.authorizer["saml_login"])
	}
}
