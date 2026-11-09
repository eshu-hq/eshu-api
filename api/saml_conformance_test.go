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
	"sync/atomic"
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

	// Exact table membership: the mirror reads the live table, so a
	// route add fails here first instead of silently riding a leg.
	want := []string{d9SAMLMetadata, d9SAMLLogin, d9SAMLACS}
	if len(d9SAMLRoutes) != len(want) {
		t.Fatalf("len(d9SAMLRoutes) = %d, want %d (the frozen triple; a route add must update this test and samlTerminatedHandler)", len(d9SAMLRoutes), len(want))
	}
	paths := map[string]string{
		d9SAMLMetadata: "/api/v0/auth/saml/providers/{provider_id}/metadata",
		d9SAMLLogin:    "/api/v0/auth/saml/providers/{provider_id}/login",
		d9SAMLACS:      "/api/v0/auth/saml/providers/{provider_id}/acs",
	}
	for i, route := range d9SAMLRoutes {
		if route != want[i] {
			t.Errorf("d9SAMLRoutes[%d] = %q, want %q (registration order is frozen)", i, route, want[i])
		}
		path, ok := paths[route]
		if !ok {
			t.Errorf("route %q: not a known terminated SAML route", route)
			continue
		}
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

// TestSAMLTermUnknownRouteFailsClosed pins the fail-closed default:
// a route the terminated table does not know answers 404, never a
// leg handler. A future table add without a handler case fails here
// instead of silently riding ACS.
func TestSAMLTermUnknownRouteFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	for _, route := range []string{
		"GET /api/v0/auth/saml/providers/acme-munich/extra",
		"POST /api/v0/auth/saml/providers/acme-munich/acs/extra",
		"DELETE /api/v0/auth/saml/providers/acme-munich/acs",
	} {
		rec := httptest.NewRecorder()
		srv.samlTerminatedHandler(route)(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("route %q: status = %d, want 404 (unknown terminated route)", route, rec.Code)
		}
	}
}

// TestSAMLTermOversizeACSFilesDenied pins the dest-cap signal: an
// oversize ACS form fails closed with zero upstream contact and files
// the denied outcome — never the dead-core local_503.
func TestSAMLTermOversizeACSFilesDenied(t *testing.T) {
	t.Parallel()

	var hits atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	tel := NewTelemetry(nil)
	srv.WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	big := strings.Repeat("A", samlTermMaxBody+1)
	req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("oversize ACS: status = %d, want permanent local 503", rec.Code)
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("oversize ACS: upstream hits = %d, want 0 (zero-contact dest-cap refusal)", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, "size bound") {
		t.Errorf("oversize ACS: body = %q, want the dest-cap detail (not a core-fetch claim)", body)
	}
	snap := tel.snapshot()
	if got := snap.dispatches["saml_login"]["native"]["denied"]; got != 1 {
		t.Errorf("oversize ACS: denied rows = %d, want 1 (zero-contact dest-cap refusal)", got)
	}
	if got := snap.dispatches["saml_login"]["native"]["local_503"]; got != 0 {
		t.Errorf("oversize ACS: local_503 rows = %d, want 0 (not a core outage)", got)
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
