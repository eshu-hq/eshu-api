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

// D9 SAML-login tier-C clone dispatch proof (Refs eshu-hq/eshu#9669,
// part of #6707; pin SAMLLoginAdmission v0). The clone is HTTP
// write-through: method, path, query, headers, and body forward to the
// same core route, and core bytes come back unmodified. Dest keeps no
// provider material, session, replay reservation, or request record,
// mints no session, keeps no ledger row, appends no audit event, and
// runs no finalize — the request creation, assertion verification,
// reserve/mint pair runs inside the core handler on every forwarded
// request.

// d9SAMLPublicCases covers the 3 pre-auth public carve-out routes with
// concrete request shapes: metadata carries the provider path value,
// login carries the provider path value plus return_to, ACS carries
// the opaque IdP-posted form bytes.
var d9SAMLPublicCases = []struct {
	name        string
	method      string
	path        string
	query       string
	body        string
	contentType string
}{
	{"metadata", http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/metadata", "", "", ""},
	{"login start", http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login", "return_to=%2Fconsole", "", ""},
	{"acs", http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", "", "RelayState=opaque-relay& SAMLResponse=opaque-response", "application/x-www-form-urlencoded"},
}

// TestD9SAMLNative_RedirectUnfollowed pins the redirect contract by
// test: a core 302 relays as a 302 with its Location, and dest never
// issues the follow-up fetch. The Location points back at the same
// stub, so a followed redirect would land a second hit; exactly one hit
// plus the relayed 302 proves dispatchClient's ErrUseLastResponse holds
// on this lane. The ACS 303 rides the same client and is covered in
// the write-through rows below.
func TestD9SAMLNative_RedirectUnfollowed(t *testing.T) {
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
		t.Errorf("Location = %q, want the core's value relayed", got)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (no follow-up fetch)", core.hits)
	}
}

// TestD9SAMLNative_WriteThroughRelaysCoreBytes pins the native
// write-through: method, path, query, headers, and body forward to the
// same core route, and the core's status, headers, and body come back
// untouched across the pinned outcomes — the metadata 200 with its
// XML content type, the 302 login start, the 201 ACS with both session
// cookies, the 303 ACS with cookies plus Location, duplicate
// deliveries, the unmounted/unknown-provider 404, the 400/401
// refusals, the core 500, and the two 503 shapes (whose Retry-After
// relays exactly as the core sent it, proving dest never invents or
// strips the core's retry signal). The provider path value, the
// return_to query value, and the ACS form fields cross opaque and
// byte-identical: dest parses, logs, and telemeters nothing. Every
// case runs in both negotiated shapes.
//
// Core bodies below are representative wire strings where core owns
// the text; the relay — not the text — is what this test pins.
func TestD9SAMLNative_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	sessionCookies := http.Header{"Set-Cookie": []string{
		"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict",
		"__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict",
	}}
	cases := []struct {
		name   string
		route  int // index into d9SAMLPublicCases
		status int
		header http.Header
		body   string
	}{
		{"metadata 200", 0, http.StatusOK, http.Header{"Content-Type": []string{"application/samlmetadata+xml"}}, `<EntityDescriptor entityID="https://eshu.example/saml/acme-munich"/>`},
		{"metadata unknown-provider 404", 0, http.StatusNotFound, nil, `{"error":"Not Found","detail":"saml provider not found"}`},
		{"start 302", 1, http.StatusFound, http.Header{"Location": []string{"https://idp.example.com/sso?SAMLRequest=opaque"}}, ``},
		{"start duplicate 302", 1, http.StatusFound, http.Header{"Location": []string{"https://idp.example.com/sso?SAMLRequest=opaque-2"}}, ``},
		{"start unknown-provider 404", 1, http.StatusNotFound, nil, `{"error":"Not Found","detail":"saml provider not found"}`},
		{"acs 201", 2, http.StatusCreated, sessionCookies, `{"csrf_token":"opaque-csrf"}`},
		{"acs duplicate 401", 2, http.StatusUnauthorized, nil, `{"error":"Unauthorized","detail":"authentication is required"}`},
		{"acs 303", 2, http.StatusSeeOther, http.Header{
			"Location":   []string{"/console"},
			"Set-Cookie": sessionCookies["Set-Cookie"],
		}, ``},
		{"acs refused 400", 2, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"RelayState and SAMLResponse are required"}`},
		{"acs refused 401", 2, http.StatusUnauthorized, nil, `{"error":"Unauthorized","detail":"authentication is required"}`},
		{"core 500", 2, http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"saml login failed"}`},
		{"core 503 retryable", 2, http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
		{"core 503 permanent", 1, http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the session store is not configured"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				route := d9SAMLPublicCases[tc.route]
				core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, skeletonTestKey)
				handler := srv.Handler(DefaultV0Sunset)

				target := route.path
				if route.query != "" {
					target += "?" + route.query
				}
				var req *http.Request
				if route.body != "" {
					req = httptest.NewRequest(route.method, target, strings.NewReader(route.body))
					req.Header.Set("Content-Type", route.contentType)
				} else {
					req = httptest.NewRequest(route.method, target, nil)
				}
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Fatalf("%s Accept %q: upstream hits = %d, want exactly 1 (no retry on write-through)", tc.name, accept, core.hits)
				}
				if core.lastMethod != route.method {
					t.Errorf("%s Accept %q: upstream method = %q, want %s", tc.name, accept, core.lastMethod, route.method)
				}
				if core.lastPath != route.path {
					t.Errorf("%s Accept %q: upstream path = %q, want the core route %q", tc.name, accept, core.lastPath, route.path)
				}
				if core.lastQuery != route.query {
					t.Errorf("%s Accept %q: upstream query = %q, want the opaque caller query %q", tc.name, accept, core.lastQuery, route.query)
				}
				if core.lastBody != route.body {
					t.Errorf("%s Accept %q: upstream body = %q, want the opaque caller body %q", tc.name, accept, core.lastBody, route.body)
				}
				if core.lastAuth != "" {
					t.Errorf("%s Accept %q: upstream Authorization = %q, want empty (no shared-key injection on public routes)", tc.name, accept, core.lastAuth)
				}
				if rec.Code != tc.status {
					t.Errorf("%s Accept %q: status = %d, want relayed %d", tc.name, accept, rec.Code, tc.status)
				}
				if loc := tc.header.Get("Location"); loc != "" {
					if got := rec.Header().Get("Location"); got != loc {
						t.Errorf("%s Accept %q: Location = %q, want the core's value relayed %q", tc.name, accept, got, loc)
					}
				}
				if cookies := tc.header.Values("Set-Cookie"); len(cookies) != 0 {
					if got := rec.Result().Header.Values("Set-Cookie"); !equalHeaders(got, cookies) {
						t.Errorf("%s Accept %q: Set-Cookie = %q, want every core cookie relayed %q", tc.name, accept, got, cookies)
					}
				}
				if tc.header.Get("Retry-After") != "" {
					if got := rec.Header().Get("Retry-After"); got != tc.header.Get("Retry-After") {
						t.Errorf("%s Accept %q: Retry-After = %q, want the core's value relayed", tc.name, accept, got)
					}
				}
				if ctype := tc.header.Get("Content-Type"); ctype != "" {
					if got := rec.Header().Get("Content-Type"); got != ctype {
						t.Errorf("%s Accept %q: Content-Type = %q, want the core's value relayed %q", tc.name, accept, got, ctype)
					}
				}
				respBody, _ := io.ReadAll(rec.Result().Body)
				if string(respBody) != tc.body {
					t.Errorf("%s Accept %q: body = %q, want the core bytes %q", tc.name, accept, respBody, tc.body)
				}
			}
		})
	}
}
