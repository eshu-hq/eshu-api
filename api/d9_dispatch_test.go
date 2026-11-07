// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// D9 OIDC-login tier-C clone dispatch proof (Refs eshu-hq/eshu#9669,
// part of #6707; pin OIDCLoginAdmission v0). The clone is HTTP
// write-through: method, path, query, headers, and body forward to the
// same core route, and core bytes come back unmodified. Dest keeps no
// OIDC state, mints no session, keeps no ledger row, appends no audit
// event, and runs no finalize — the state creation, verification,
// mint/verify/append/finalize pair runs inside the core handler on
// every forwarded request.

// d9PublicCases covers the 2 pre-auth public carve-out routes with
// concrete request shapes: start carries the provider IDs plus
// return_to, callback carries the credential state + code.
var d9PublicCases = []struct {
	name   string
	method string
	path   string
	query  string
}{
	{"login start", http.MethodGet, "/api/v0/auth/oidc/login", "provider_config_id=okta-dev&tenant_id=tenant_a&workspace_id=workspace_a&return_to=%2Fconsole"},
	{"login callback", http.MethodGet, "/api/v0/auth/oidc/callback", "state=st-opaque-123&code=cd-opaque-456"},
}

// TestD9Native_RedirectUnfollowed pins the redirect contract by test:
// a core 302 relays as a 302 with its Location, and dest never issues
// the follow-up fetch. The Location points back at the same stub, so a
// followed redirect would land a second hit; exactly one hit plus the
// relayed 302 proves dispatchClient's ErrUseLastResponse holds on this
// lane. The callback 303 rides the same client and is covered in the
// write-through rows below.
func TestD9Native_RedirectUnfollowed(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusFound,
		header: http.Header{"Location": []string{"/api/v0/auth/oidc/callback?state=st-opaque-123&code=cd-opaque-456"}},
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/login?provider_config_id=okta-dev", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want the core's 302 unfollowed", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/api/v0/auth/oidc/callback?state=st-opaque-123&code=cd-opaque-456" {
		t.Errorf("Location = %q, want the core's value relayed", got)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (no follow-up fetch)", core.hits)
	}
}

// TestD9Native_WriteThroughRelaysCoreBytes pins the native
// write-through: method, path, query, and headers forward to the same
// core route, and the core's status, headers, and body come back
// untouched across the pinned outcomes — the 302 start redirect, the
// 201 callback with both session cookies, the 303 callback with
// cookies plus Location, duplicate deliveries, the 503 unconfigured
// refusal, the 400/401/403 refusals, the core 500, and the two 503
// shapes (whose Retry-After relays exactly as the core sent it,
// proving dest never invents or strips the core's retry signal). The
// credential query values cross opaque and byte-identical: dest
// parses, logs, and telemeters nothing. Every case runs in both
// negotiated shapes.
//
// Core bodies below are representative wire strings where core owns
// the text; the relay — not the text — is what this test pins.
func TestD9Native_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	sessionCookies := http.Header{"Set-Cookie": []string{
		"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict",
		"__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict",
	}}
	cases := []struct {
		name   string
		route  int // index into d9PublicCases
		status int
		header http.Header
		body   string
	}{
		{"start 302", 0, http.StatusFound, http.Header{"Location": []string{"https://idp.example.com/authorize?state=opaque&nonce=opaque"}}, ``},
		{"start duplicate 302", 0, http.StatusFound, http.Header{"Location": []string{"https://idp.example.com/authorize?state=opaque-2&nonce=opaque-2"}}, ``},
		{"start refused 503", 0, http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"oidc login is unavailable"}`},
		{"start refused 400", 0, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid oidc login request"}`},
		{"callback 201", 1, http.StatusCreated, sessionCookies, `{"csrf_token":"opaque-csrf"}`},
		{"callback duplicate 201", 1, http.StatusCreated, sessionCookies, `{"csrf_token":"opaque-csrf"}`},
		{"callback 303", 1, http.StatusSeeOther, http.Header{
			"Location":   []string{"/console"},
			"Set-Cookie": sessionCookies["Set-Cookie"],
		}, ``},
		{"callback refused 400", 1, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid oidc login request"}`},
		{"callback refused 401", 1, http.StatusUnauthorized, nil, `{"error":"Unauthorized","detail":"authentication is required"}`},
		{"callback refused 403", 1, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"oidc login denied"}`},
		{"core 500", 0, http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"oidc login failed"}`},
		{"core 503 retryable", 1, http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
		{"core 503 permanent", 0, http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the session store is not configured"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				route := d9PublicCases[tc.route]
				core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, skeletonTestKey)
				handler := srv.Handler(DefaultV0Sunset)

				target := route.path + "?" + route.query
				req := httptest.NewRequest(route.method, target, nil)
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
				if core.lastBody != "" {
					t.Errorf("%s Accept %q: upstream body = %q, want empty (login GETs carry no body)", tc.name, accept, core.lastBody)
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
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != tc.body {
					t.Errorf("%s Accept %q: body = %q, want the core bytes %q", tc.name, accept, body, tc.body)
				}
			}
		})
	}
}
