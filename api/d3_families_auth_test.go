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

// D3 per-family authorizer proof (Refs eshu-hq/eshu#9669, part of
// #6707). Scope-honesty label: the dest authorizer covers the
// shared-key (AllScopes) identity only — the single class dest can
// construct today. Scoped tokens, OIDC bearers, and browser sessions
// relay explicitly (core judges;
// TestD3WaveRelayExplicitForUnresolvableIdentities pins that, and
// TestD3WaveJunkCookieStays401 pins that junk cookies never relay),
// and the deny branch below is unreachable on live routes until the D8
// session resolver lands. Core re-judges every forwarded call as
// backstop (tenant scope, permission catalog, governance mode), which
// is where the live 403s come from.

// TestD3AuthorizerSharedKeyAllowDeny pins the family gate: the
// shared-key identity allows, anything else denies. One shared
// authorizer covers all 16 D3 wave routes until D8 resolves per-class
// handling.
func TestD3AuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeD3SharedKey(true) {
		t.Errorf("authorizeD3SharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeD3SharedKey(false) {
		t.Errorf("authorizeD3SharedKey(false) = allow, want deny (family 403)")
	}
}

// TestD3AuthorizerCountersPinned pins the authorizer telemetry for
// the wave: a terminated shared-key call files one allow judgment on
// its family, and the unreachable deny branch stays observable. One
// representative route per D3 family.
func TestD3AuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		method string
		path   string
		family string
	}{
		{http.MethodPost, "/api/v0/admin/work-items/query", "admin"},
		{http.MethodGet, "/api/v0/auth/admin/roles", "admin_identity"},
		{http.MethodGet, "/api/v0/auth/admin/audit/summary", "governance_audit"},
		{http.MethodPost, "/api/v0/admin/collector-failures/query", "terminal_failure"},
	} {
		core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		tel := NewTelemetry(nil)
		srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(tc.method, tc.path)
		if tc.method == http.MethodPost {
			req.Body = io.NopCloser(http.NoBody)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: status = %d, want 200", tc.method, tc.path, rec.Code)
		}
		snap := tel.snapshot()
		if got := snap.authorizer[tc.family][decisionAllow]; got != 1 {
			t.Errorf("%s %s: authorizer[%s][allow] = %d, want 1", tc.method, tc.path, tc.family, got)
		}
		if got := snap.authorizer[tc.family][decisionDeny]; got != 0 {
			t.Errorf("%s %s: authorizer[%s][deny] = %d, want 0 on the allow path", tc.method, tc.path, tc.family, got)
		}
	}
}

// TestD3DenyBranchShapePinned pins the deny shape for the day dest
// gains a scoped/session resolver: a non-AllScopes identity is denied
// with the documented family 403 in both negotiated shapes and the
// core is never reached. The branch is unreachable on the live routes
// today (the wave gate passes true; the live 403s are core's), so this
// pins the contract, not a live path.
func TestD3DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v0/admin/work-items/query"},
		{http.MethodPost, "/api/v0/admin/decisions/query"},
		{http.MethodPost, "/api/v0/admin/dead-letters/query"},
		{http.MethodPost, "/api/v0/admin/input-invalid-facts/query"},
		{http.MethodPost, "/api/v0/admin/changed-since/poisoned-links/query"},
		{http.MethodPost, "/api/v0/admin/replay-events/query"},
		{http.MethodGet, "/api/v0/auth/admin/api-tokens"},
		{http.MethodGet, "/api/v0/auth/admin/idp-group-mappings"},
		{http.MethodGet, "/api/v0/auth/admin/idp-providers"},
		{http.MethodGet, "/api/v0/auth/admin/role-assignments"},
		{http.MethodGet, "/api/v0/auth/admin/roles"},
		{http.MethodGet, "/api/v0/auth/local/invitations"},
		{http.MethodGet, "/api/v0/auth/admin/audit/events"},
		{http.MethodGet, "/api/v0/auth/admin/audit/summary"},
		{http.MethodPost, "/api/v0/admin/collector-failures/acknowledgements/query"},
		{http.MethodPost, "/api/v0/admin/collector-failures/query"},
	}
	if len(cases) != 16 {
		t.Fatalf("deny-shape cases = %d, want the 16 D3 wave routes", len(cases))
	}
	for _, tc := range cases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(tc.method, tc.path)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			srv.serveD3TerminatedAs(rec, req, false)

			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s Accept %q: status = %d, want 403", tc.method, tc.path, accept, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), d3FamilyDenyMessage) {
				t.Errorf("%s %s Accept %q: body = %q, want the family-403 message", tc.method, tc.path, accept, body)
			}
		}
	}
}

// TestD3Gated_UnauthenticatedIs401 pins that every D3 wave route keeps
// the dest 401 shape: envelope callers get the envelope error,
// everyone else the plain error_code body, both with the bare Bearer
// challenge — and the core is never contacted.
func TestD3Gated_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, entry := range d3Wave {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			key := entry.Method + " " + entry.Path
			req := httptest.NewRequest(entry.Method, d3ConcretePaths[key], nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s Accept %q: status = %d, want 401", entry.Method, entry.Path, accept, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("%s %s Accept %q: WWW-Authenticate = %q, want bare Bearer", entry.Method, entry.Path, accept, got)
			}
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (gate runs before dispatch)", core.hits)
	}
}

// TestD3Gated_UnconfiguredOriginFailsClosed pins the fail-closed
// origin on the terminated path: without a valid core origin every D3
// call answers the permanent local 503 (no Retry-After) in both wire
// shapes.
func TestD3Gated_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"", "://missing-scheme-host"} {
		srv := NewServer(origin, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(http.MethodPost, "/api/v0/admin/work-items/query")
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("origin %q Accept %q: status = %d, want 503", origin, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("origin %q Accept %q: Retry-After is set on the permanent 503", origin, accept)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), "backend_unavailable") {
				t.Errorf("origin %q Accept %q: body = %s, want backend_unavailable", origin, accept, body)
			}
		}
	}
}

// TestD3Gated_TrailingSlashStaysNative pins the never-split rule: the
// trailing-slash spelling of each D3 wave route rides the wave gate
// (shared-key terminates on the native lane), not the generic relay.
func TestD3Gated_TrailingSlashStaysNative(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	targets := []struct{ method, path string }{
		{http.MethodPost, "/api/v0/admin/work-items/query/"},
		{http.MethodGet, "/api/v0/auth/admin/roles/"},
		{http.MethodGet, "/api/v0/auth/admin/audit/summary/"},
		{http.MethodPost, "/api/v0/admin/collector-failures/query/"},
	}
	for _, tc := range targets {
		req := authedRequest(tc.method, tc.path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s %s: status = %d, want 200", tc.method, tc.path, rec.Code)
		}
		if core.lastPath != tc.path {
			t.Errorf("%s %s: upstream path = %q, want the slash spelling forwarded", tc.method, tc.path, core.lastPath)
		}
	}
	snap := tel.snapshot()
	for _, family := range []string{"admin", "admin_identity", "governance_audit", "terminal_failure"} {
		if got := snap.dispatches[family][viaNative][outcomeCoreOK]; got < 1 {
			t.Errorf("family %q native core_ok = %d, want >= 1 (slash spelling on the native lane)", family, got)
		}
	}
}

// TestD3Gated_V1AliasTerminates pins that the v1 alias rewrite lands
// D3 wave routes on the same native handler: the core sees the
// rewritten v0 path and the response carries no deprecation headers.
func TestD3Gated_V1AliasTerminates(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range []struct{ method, v1, v0 string }{
		{http.MethodGet, "/api/v1/auth/admin/roles", "/api/v0/auth/admin/roles"},
		{http.MethodPost, "/api/v1/admin/work-items/query", "/api/v0/admin/work-items/query"},
	} {
		req := authedRequest(tc.method, tc.v1)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: status = %d, want 200", tc.method, tc.v1, rec.Code)
		}
		if core.lastPath != tc.v0 {
			t.Errorf("%s %s: upstream path = %q, want the rewritten %s", tc.method, tc.v1, core.lastPath, tc.v0)
		}
		if got := rec.Header().Get("Deprecation"); got != "" {
			t.Errorf("%s %s: Deprecation = %q, want empty on v1", tc.method, tc.v1, got)
		}
	}
}
