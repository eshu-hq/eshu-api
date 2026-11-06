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

// providerConfigNativeCases covers the six tier C-b routes with concrete
// request paths (wildcards filled): method and path forward verbatim.
// The create and update POSTs share path keys with the family list and
// detail GETs; Go matches method and path together, so the POSTs ride
// the native route while those GETs stay on the generic relay (pinned
// by TestProviderConfigNative_ReadsStillProxy below).
var providerConfigNativeCases = []struct {
	name   string
	method string
	path   string
}{
	{"create", http.MethodPost, "/api/v0/auth/admin/provider-configs"},
	{"update", http.MethodPost, "/api/v0/auth/admin/provider-configs/prov_1"},
	{"revert", http.MethodPost, "/api/v0/auth/admin/provider-configs/prov_1/revert"},
	{"enable", http.MethodPost, "/api/v0/auth/admin/provider-configs/prov_1/enable"},
	{"disable", http.MethodPost, "/api/v0/auth/admin/provider-configs/prov_1/disable"},
	{"test connection", http.MethodPost, "/api/v0/auth/admin/provider-configs/prov_1/test-connection"},
}

// TestProviderConfigNative_UnauthenticatedIs401 pins that every native
// route keeps the dest 401 shape: envelope callers get the envelope
// error, everyone else the plain error_code body, both with the bare
// Bearer [REDACTED]
func TestProviderConfigNative_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range providerConfigNativeCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s Accept %q: status = %d, want 401", tc.name, accept, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("%s Accept %q: WWW-Authenticate = %q, want bare Bearer", tc.name, accept, got)
			}
		}
	}
}

// TestProviderConfigNative_AuthorizerRejectsNonAllScopes pins the family
// 403 the dest authorizer adds: a non-AllScopes identity never reaches
// the core. Dest has no scoped/session resolver, so the deny branch is
// unreachable in production (the shared key maps to AllScopes in core,
// which re-judges tenant scope and the
// identity_admin.provider_config_* permission catalog as backstop); the
// test pins the documented shape, not a live path.
func TestProviderConfigNative_AuthorizerRejectsNonAllScopes(t *testing.T) {
	t.Parallel()

	if !authorizeProviderConfigAdmin(true) {
		t.Errorf("authorizeProviderConfigAdmin(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeProviderConfigAdmin(false) {
		t.Errorf("authorizeProviderConfigAdmin(false) = allow, want deny (family 403)")
	}
}

// TestProviderConfigNative_DenyBranchShapePinned pins the deny shape for
// the day dest gains a scoped/session resolver: a non-AllScopes identity
// is denied with the documented shape and the core is never reached, in
// both negotiated shapes. The branch is unreachable on the live routes
// today (the routes pass true; the live 403s are core's), so this pins
// the contract, not a live path.
func TestProviderConfigNative_DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, tc := range providerConfigNativeCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(tc.method, tc.path)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			srv.providerConfigWriteThroughAs(rec, req, false)

			if rec.Code != http.StatusForbidden {
				t.Errorf("%s Accept %q: status = %d, want 403", tc.name, accept, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), "all-scopes operator authorization is required") {
				t.Errorf("%s Accept %q: body = %q, want the family-403 message", tc.name, accept, body)
			}
		}
	}
}

// TestProviderConfigNative_WriteThroughRelaysCoreBytes pins the native
// write-through: method, path, query, headers (including the original
// Authorization header), and body forward to the same core route, and
// the core's status, headers, and body come back untouched across the
// pinned outcomes — 200 results and idempotent-retry no-ops, the 400
// validation refusals, the core 403s (all-scope, tenant scope,
// permission catalog), the 404s, the duplicate-key and revision-changed
// 409s (whose single upstream hit proves dest never retries a
// conflict), and the core 503 (whose Retry-After relays, proving dest
// never invents or strips the core's retry signal). Every case runs in
// both negotiated shapes.
//
// Core bodies below are the pinned wire strings where core owns the
// text (WriteError refusals from the in-tree handlers at the provenance
// pin); the permission-catalog denial is opaque core bytes whose relay
// — not whose text — this test pins.
func TestProviderConfigNative_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	createPayload := `{"provider_kind":"oidc","display_name":"Corp IdP"}`
	revertPayload := `{"revision_id":"rev_0"}`
	created := `{"provider_config_id":"prov_1","revision_id":"rev_1","status":"draft","changed":true}`
	reverted := `{"provider_config_id":"prov_1","revision_id":"rev_0","status":"draft","changed":true}`
	revertNoop := `{"provider_config_id":"prov_1","revision_id":"rev_1","status":"draft","changed":false}`
	enabled := `{"provider_config_id":"prov_1","revision_id":"rev_1","status":"active","changed":true}`
	disabled := `{"provider_config_id":"prov_1","revision_id":"rev_1","status":"draft","changed":true}`
	cases := []struct {
		name    string
		route   int // index into providerConfigNativeCases
		status  int
		header  http.Header
		body    string
		payload string
		query   string
	}{
		{"create", 0, http.StatusOK, nil, created, createPayload, ""},
		{"create duplicate 409", 0, http.StatusConflict, nil, `{"error":"Conflict","detail":"a provider config already exists for this tenant, kind, and identity key"}`, createPayload, ""},
		{"update", 1, http.StatusOK, nil, created, createPayload, ""},
		{"update missing 404", 1, http.StatusNotFound, nil, `{"error":"Not Found","detail":"provider config not found"}`, createPayload, ""},
		{"revert", 2, http.StatusOK, nil, reverted, revertPayload, ""},
		{"revert noop", 2, http.StatusOK, nil, revertNoop, revertPayload, ""},
		{"revert invalid 400", 2, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid revert request"}`, "{}", ""},
		{"revert revision 404", 2, http.StatusNotFound, nil, `{"error":"Not Found","detail":"revision not found"}`, revertPayload, ""},
		{"enable", 3, http.StatusOK, nil, enabled, "", ""},
		{"enable test failed 400", 3, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"provider cannot be enabled: connection test did not pass"}`, "", ""},
		{"enable revision changed 409", 3, http.StatusConflict, nil, `{"error":"Conflict","detail":"the provider config's active revision changed since it was tested; run test-connection again and retry enable"}`, "", ""},
		{"enable missing 404", 3, http.StatusNotFound, nil, `{"error":"Not Found","detail":"provider config not found"}`, "", ""},
		{"disable", 4, http.StatusOK, nil, disabled, "", ""},
		{"disable missing 404", 4, http.StatusNotFound, nil, `{"error":"Not Found","detail":"provider config not found"}`, "", ""},
		{"test connection pass", 5, http.StatusOK, nil, `{"provider_config_id":"prov_1","ok":true,"detail":"reachable"}`, "", ""},
		{"test connection fail", 5, http.StatusOK, nil, `{"provider_config_id":"prov_1","ok":false,"detail":"dial timeout"}`, "", ""},
		{"test connection no tester 503", 5, http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"connection test capability is unavailable"}`, "", ""},
		{"core all-scope 403", 0, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`, createPayload, ""},
		{"tenant scope 403", 3, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"admin tenant scope is required"}`, "", ""},
		{"catalog 403 relays opaquely", 4, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"denied by the permission catalog"}`, "", ""},
		{"core 503 retryable", 2, http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"admin provider config mutation store is unavailable"}`, revertPayload, "workspace_id=ws_1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				route := providerConfigNativeCases[tc.route]
				core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, skeletonTestKey)
				handler := srv.Handler(DefaultV0Sunset)

				target := route.path
				if tc.query != "" {
					target += "?" + tc.query
				}
				req := authedRequest(route.method, target)
				req.Header.Set("Accept", accept)
				if tc.payload != "" {
					req.Body = io.NopCloser(strings.NewReader(tc.payload))
				}
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
				if core.lastQuery != tc.query {
					t.Errorf("%s Accept %q: upstream query = %q, want %q", tc.name, accept, core.lastQuery, tc.query)
				}
				if core.lastBody != tc.payload {
					t.Errorf("%s Accept %q: upstream body = %q, want the caller bytes", tc.name, accept, core.lastBody)
				}
				if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
					t.Errorf("%s Accept %q: upstream Authorization = %q, want the forwarded Bearer [REDACTED]", tc.name, accept, core.lastAuth)
				}
				if rec.Code != tc.status {
					t.Errorf("%s Accept %q: status = %d, want relayed %d", tc.name, accept, rec.Code, tc.status)
				}
				if tc.header != nil {
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

// TestProviderConfigNative_ReadsStillProxy pins that the native mutation
// routes change nothing for the family's reads: the list, detail, and
// revisions GETs still cross the generic byte-relay with path, method,
// and query intact. The create and update POSTs share those path keys
// but ride the native route by method; the reads-split follow-up ports
// their fragment entries alongside the GETs.
func TestProviderConfigNative_ReadsStillProxy(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/auth/admin/provider-configs",
		"/api/v0/auth/admin/provider-configs/prov_1",
		"/api/v0/auth/admin/provider-configs/prov_1/revisions",
	} {
		core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
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
