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

// D8 session-resolver tier-C clone dispatch proof (Refs
// eshu-hq/eshu#9669, part of #6707; pins SessionResolver,
// BrowserSession, LocalIdentity, APIToken v0). The clone is HTTP
// write-through: method, path, query, headers (including the original
// Authorization header), and body forward to the same core route, and
// core bytes come back unmodified. Dest mints no session, token, or
// credential, keeps no ledger row, appends no audit event, and runs no
// finalize — the mint/verify/append/finalize pair runs inside the core
// handler on every forwarded request.

// d8GatedCases covers the 18 authed tier-C clone routes with concrete
// request paths: {user_id} and {token_id} ride representative values so
// the path-param spellings prove the write-through too.
var d8GatedCases = []struct {
	name   string
	method string
	path   string
}{
	{"browser create", http.MethodPost, "/api/v0/auth/browser-session"},
	{"browser read", http.MethodGet, "/api/v0/auth/browser-session"},
	{"browser revoke", http.MethodDelete, "/api/v0/auth/browser-session"},
	{"browser context", http.MethodPatch, "/api/v0/auth/browser-session/context"},
	{"session list", http.MethodGet, "/api/v0/auth/sessions"},
	{"profile", http.MethodGet, "/api/v0/auth/profile"},
	{"bootstrap", http.MethodPost, "/api/v0/auth/local/bootstrap"},
	{"invitations create", http.MethodPost, "/api/v0/auth/local/invitations"},
	{"password reset", http.MethodPost, "/api/v0/auth/local/users/u-1/password"},
	{"mfa reset", http.MethodPost, "/api/v0/auth/local/users/u-1/mfa-reset"},
	{"disable", http.MethodPost, "/api/v0/auth/local/users/u-1/disable"},
	{"break-glass", http.MethodPost, "/api/v0/auth/local/break-glass"},
	{"api-tokens list", http.MethodGet, "/api/v0/auth/local/api-tokens"},
	{"api-tokens create", http.MethodPost, "/api/v0/auth/local/api-tokens"},
	{"api-token revoke", http.MethodPost, "/api/v0/auth/local/api-tokens/t-1/revoke"},
	{"api-token rotate", http.MethodPost, "/api/v0/auth/local/api-tokens/t-1/rotate"},
	{"totp begin", http.MethodPost, "/api/v0/auth/local/mfa/totp/begin"},
	{"totp confirm", http.MethodPost, "/api/v0/auth/local/mfa/totp/confirm"},
}

// d8PublicCases covers the 4 pre-auth public carve-out routes with
// concrete request paths.
var d8PublicCases = []struct {
	name   string
	method string
	path   string
}{
	{"login", http.MethodPost, "/api/v0/auth/local/login"},
	{"invitations accept", http.MethodPost, "/api/v0/auth/local/invitations/accept"},
	{"break-glass session", http.MethodPost, "/api/v0/auth/local/break-glass/session"},
	{"password rotate", http.MethodPost, "/api/v0/auth/local/password/rotate"},
}

// TestD8Native_UnauthenticatedIs401 pins that every gated D8 route
// keeps the dest 401 shape: envelope callers get the envelope error,
// everyone else the plain error_code body, both with the bare Bearer [REDACTED]
// The 4 public carve-out routes are NOT here: they serve keyless
// callers by design (pinned in TestD8Public_CarveOutServesKeyless).
func TestD8Native_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range d8GatedCases {
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

// TestD8Native_AuthorizerRejectsNonAllScopes pins the family 403 the
// dest authorizer adds: a non-AllScopes identity never reaches the
// core. Dest has no session resolver, so the deny branch is
// unreachable in production (the shared key maps to AllScopes in core,
// which re-judges tenant scope, the permission catalog, and the
// session/cookie judgments as backstop); the test pins the documented
// shape, not a live path.
func TestD8Native_AuthorizerRejectsNonAllScopes(t *testing.T) {
	t.Parallel()

	if !authorizeD8(true) {
		t.Errorf("authorizeD8(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeD8(false) {
		t.Errorf("authorizeD8(false) = allow, want deny (family 403)")
	}
}

// TestD8Native_DenyBranchShapePinned pins the deny shape for the day
// dest gains a session resolver: a non-AllScopes identity is denied
// with the documented shape and the core is never reached, in both
// negotiated shapes. The branch is unreachable on the live routes
// today (the gate passes true; the live 403s and 401s are core's), so
// this pins the contract, not a live path.
func TestD8Native_DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, tc := range d8GatedCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(tc.method, tc.path)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			srv.d8WriteThroughAs(rec, req, false)

			if rec.Code != http.StatusForbidden {
				t.Errorf("%s Accept %q: status = %d, want 403", tc.name, accept, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), d8FamilyDenyMessage) {
				t.Errorf("%s Accept %q: body = %q, want the family-403 message", tc.name, accept, body)
			}
		}
	}
}

// TestD8Native_WriteThroughRelaysCoreBytes pins the native
// write-through: method, path, query, headers (including the original
// Authorization header), and body forward to the same core route, and
// the core's status, headers, and body come back untouched across the
// pinned outcomes — 200/201/202/204 results, duplicate deliveries, the
// 400 validation refusals, the 401 authentication refusals, the core
// 403s and 404s, the core 500, and the two 503 shapes (whose
// Retry-After relays exactly as the core sent it, proving dest never
// invents or strips the core's retry signal). Credential-bearing POST
// bodies cross opaque and byte-identical: dest parses, logs, and
// telemeters nothing. Every case runs in both negotiated shapes.
//
// Core bodies below are representative wire strings where core owns
// the text; the relay — not the text — is what this test pins.
func TestD8Native_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		route   int // index into d8GatedCases
		status  int
		header  http.Header
		body    string
		payload string
		query   string
	}{
		{"create 201", 0, http.StatusCreated, nil, `{"status":"created"}`, `{"ttl":"8h"}`, ""},
		{"create refused 400", 0, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"explicit credential is required"}`, `{"ttl":"8h"}`, ""},
		{"create duplicate 201", 0, http.StatusCreated, nil, `{"status":"created"}`, `{"ttl":"8h"}`, ""},
		{"read 200", 1, http.StatusOK, nil, `{"auth":{"mode":"browser_session"}}`, "", ""},
		{"read refused 401", 1, http.StatusUnauthorized, nil, `{"error":"Unauthorized","detail":"session expired"}`, "", ""},
		{"revoke 204", 2, http.StatusNoContent, nil, ``, "", ""},
		{"context switched 200", 3, http.StatusOK, nil, `{"status":"switched"}`, `{"tenant_id":"t1","workspace_id":"w1"}`, ""},
		{"context refused 403", 3, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"scoped sessions cannot switch workspaces"}`, `{"tenant_id":"t1"}`, ""},
		{"sessions 200", 4, http.StatusOK, nil, `{"sessions":[],"prev":null,"next":null}`, "", "limit=25&offset=0"},
		{"profile 200", 5, http.StatusOK, nil, `{"subject_class":"local","mfa_active":false}`, "", ""},
		{"profile refused 401", 5, http.StatusUnauthorized, nil, `{"error":"Unauthorized","detail":"authentication is required"}`, "", ""},
		{"bootstrap 201", 6, http.StatusCreated, nil, `{"status":"bootstrapped"}`, `{"username":"owner","password":"first-proof"}`, ""},
		{"bootstrap refused 403", 6, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"already bootstrapped"}`, `{"username":"x"}`, ""},
		{"invitations create 201", 7, http.StatusCreated, nil, `{"status":"invited"}`, `{"role_id":"r1"}`, ""},
		{"password reset 200", 8, http.StatusOK, nil, `{"status":"reset"}`, `{"new_password":"rotated-proof"}`, ""},
		{"mfa reset 200", 9, http.StatusOK, nil, `{"status":"mfa_reset"}`, `{"note":"lost device"}`, ""},
		{"disable 200", 10, http.StatusOK, nil, `{"status":"disabled"}`, `{"note":"offboarded"}`, ""},
		{"break-glass 200", 11, http.StatusOK, nil, `{"status":"break_glass_enabled"}`, `{"break_glass_code":"envelope-proof"}`, ""},
		{"break-glass refused 403", 11, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"shared operator is required"}`, `{}`, ""},
		{"tokens list 200", 12, http.StatusOK, nil, `{"tokens":[]}`, "", ""},
		{"tokens create 201", 13, http.StatusCreated, nil, `{"status":"created"}`, `{"token_class":"personal"}`, ""},
		{"token revoke 200", 14, http.StatusOK, nil, `{"status":"revoked"}`, `{}`, ""},
		{"token rotate 201", 15, http.StatusCreated, nil, `{"status":"rotated"}`, `{}`, ""},
		{"token unknown 404", 14, http.StatusNotFound, nil, `{"error":"Not Found","detail":"unknown token"}`, `{}`, ""},
		{"totp begin 200", 16, http.StatusOK, nil, `{"status":"begun"}`, `{"current_password":"s3cret-proof"}`, ""},
		{"totp confirm 200", 17, http.StatusOK, nil, `{"status":"confirmed"}`, `{"code":"123456"}`, ""},
		{"totp refused 401", 17, http.StatusUnauthorized, nil, `{"error":"Unauthorized","detail":"invalid code"}`, `{"code":"000000"}`, ""},
		{"core all-scope 403", 13, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"admin authentication is required"}`, `{"token_class":"personal"}`, ""},
		{"core 500", 3, http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"store is unavailable"}`, `{"tenant_id":"t1"}`, ""},
		{"core 503 retryable", 16, http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`, `{"current_password":"s3cret-proof"}`, ""},
		{"core 503 permanent", 0, http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the session store is not configured"}`, `{"ttl":"8h"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				route := d8GatedCases[tc.route]
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
					t.Errorf("%s Accept %q: upstream Authorization = %q, want the forwarded Bearer", tc.name, accept, core.lastAuth)
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
