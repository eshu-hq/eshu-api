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

// adminIdentityNativeCases covers the five tier C-b routes with concrete
// request paths (wildcards filled): method and path forward verbatim.
var adminIdentityNativeCases = []struct {
	name   string
	method string
	path   string
}{
	{"revoke invitation", http.MethodPost, "/api/v0/auth/local/invitations/inv_1/revoke"},
	{"grant role", http.MethodPost, "/api/v0/auth/admin/role-assignments"},
	{"revoke role", http.MethodPost, "/api/v0/auth/admin/role-assignments/revoke"},
	{"create mapping", http.MethodPost, "/api/v0/auth/admin/idp-group-mappings"},
	{"delete mapping", http.MethodDelete, "/api/v0/auth/admin/idp-group-mappings/ref_1"},
}

// TestAdminIdentityNative_UnauthenticatedIs401 pins that every native
// route keeps the dest 401 shape: envelope callers get the envelope
// error, everyone else the plain error_code body, both with the bare
// Bearer challenge.
func TestAdminIdentityNative_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range adminIdentityNativeCases {
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

// TestAdminIdentityNative_AuthorizerRejectsNonAllScopes pins the family
// 403 the dest authorizer adds: a non-AllScopes identity never reaches
// the core. Dest has no scoped/session resolver, so the deny branch is
// unreachable in production (the shared key maps to AllScopes in core,
// which re-judges tenant scope, workspace match, and the
// identity_admin/roles_grants permission catalog as backstop); the test
// pins the documented shape, not a live path.
func TestAdminIdentityNative_AuthorizerRejectsNonAllScopes(t *testing.T) {
	t.Parallel()

	if !authorizeAdminIdentityAdmin(true) {
		t.Errorf("authorizeAdminIdentityAdmin(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeAdminIdentityAdmin(false) {
		t.Errorf("authorizeAdminIdentityAdmin(false) = allow, want deny (family 403)")
	}
}

// TestAdminIdentityNative_DenyBranchShapePinned pins the deny shape for
// the day dest gains a scoped/session resolver: a non-AllScopes identity
// is denied with the documented shape and the core is never reached, in
// both negotiated shapes. The branch is unreachable on the live routes
// today (the routes pass true; the live 403s are core's), so this pins
// the contract, not a live path.
func TestAdminIdentityNative_DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, tc := range adminIdentityNativeCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(tc.method, tc.path)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			srv.adminIdentityWriteThroughAs(rec, req, false)

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

// TestAdminIdentityNative_WriteThroughRelaysCoreBytes pins the native
// write-through: method, path, query, headers (including the original
// Authorization header), and body forward to the same core route, and
// the core's status, headers, and body come back untouched across the
// pinned outcomes — 200 results and idempotent-retry no-ops, the 400
// validation refusals, the core 403s (all-scope, tenant scope,
// workspace match), the invitation 404, the stale-ref 409 (whose single
// upstream hit proves dest never retries a conflict), and the core 503
// (whose Retry-After relays, proving dest never invents or strips the
// core's retry signal). Every case runs in both negotiated shapes.
func TestAdminIdentityNative_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	grantPayload := `{"user_id":"u1","role_id":"developer"}`
	createPayload := `{"provider_config_id":"prov_1","external_group":"Eshu Developers","role_id":"developer"}`
	cases := []struct {
		name    string
		route   int // index into adminIdentityNativeCases
		status  int
		header  http.Header
		body    string
		payload string
		query   string
	}{
		{"grant", 1, http.StatusOK, nil, `{"user_id":"u1","role_id":"developer","status":"active","changed":true}`, grantPayload, ""},
		{"grant reactivation", 1, http.StatusOK, nil, `{"user_id":"u1","role_id":"developer","status":"active","changed":false}`, grantPayload, ""},
		{"revoke role", 2, http.StatusOK, nil, `{"user_id":"u1","role_id":"developer","status":"revoked","changed":true}`, grantPayload, ""},
		{"revoke invitation", 0, http.StatusOK, nil, `{"invite_id":"inv_1","status":"revoked","revoked":true}`, "", ""},
		{"revoke invitation noop", 0, http.StatusOK, nil, `{"invite_id":"inv_1","status":"revoked","revoked":false}`, "", ""},
		{"create mapping", 3, http.StatusOK, nil, `{"mapping_ref":"ref_1","provider_config_id":"prov_1","role_id":"developer","status":"active","created":true}`, createPayload, "workspace_id=ws_1"},
		{"create mapping reactivation", 3, http.StatusOK, nil, `{"mapping_ref":"ref_1","provider_config_id":"prov_1","role_id":"developer","status":"active","created":false}`, createPayload, ""},
		{"delete mapping noop", 4, http.StatusOK, nil, `{"mapping_ref":"ref_1","deleted":false}`, "", ""},
		{"unknown role", 1, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"role does not exist or is not active in the tenant"}`, grantPayload, ""},
		{"core all-scope 403", 1, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`, grantPayload, ""},
		{"tenant scope 403", 1, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"admin tenant scope is required"}`, grantPayload, ""},
		{"workspace mismatch 403", 2, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"workspace_id must match the caller's workspace"}`, grantPayload, ""},
		{"invitation not found", 0, http.StatusNotFound, nil, `{"error":"Not Found","detail":"invitation not found"}`, "", ""},
		{"stale ref 409", 4, http.StatusConflict, nil, `{"error":"Conflict","detail":"mapping_ref is stale; list group mappings again and use the current ref"}`, "", ""},
		{"core 503 retryable", 3, http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"admin identity mutation store is unavailable"}`, createPayload, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				route := adminIdentityNativeCases[tc.route]
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
					t.Errorf("%s Accept %q: upstream Authorization = %q, want the forwarded Bearer key", tc.name, accept, core.lastAuth)
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

// TestAdminIdentityNative_ReadsStillProxy pins that the native mutation
// routes change nothing for the family's reads: the list GETs still
// cross the generic byte-relay with path, method, and query intact.
func TestAdminIdentityNative_ReadsStillProxy(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/auth/local/invitations",
		"/api/v0/auth/admin/role-assignments?user_id=u1",
		"/api/v0/auth/admin/roles",
		"/api/v0/auth/admin/idp-providers",
		"/api/v0/auth/admin/idp-group-mappings",
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

		wantPath := strings.SplitN(path, "?", 2)[0]
		if core.hits != 1 || core.lastPath != wantPath || core.lastMethod != http.MethodGet {
			t.Errorf("path %q: hits = %d method = %q lastPath = %q, want the generic proxy to the read route",
				path, core.hits, core.lastMethod, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("path %q: status = %d, want 200", path, rec.Code)
		}
	}
}
