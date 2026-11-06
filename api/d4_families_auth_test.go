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

// D4 Native-next per-family authorizer proof (Refs eshu-hq/eshu#9669,
// part of #6707). Scope-honesty label: the dest authorizer covers the
// shared-key (AllScopes) identity only — the single class dest can
// construct today. Scoped tokens, OIDC bearers, and browser sessions
// relay explicitly (core judges; TestD4WaveRelayExplicit... pins
// that), and the deny branch below is unreachable on live routes until
// the D8 session resolver lands. Core re-judges every forwarded call
// as backstop (tenant scope, permission catalog, governance mode),
// which is where the live 403s come from.

// TestD4AuthorizerSharedKeyAllowDeny pins the family gate: the
// shared-key identity allows, anything else denies. One shared
// authorizer covers all 46 D4 wave routes until D8 resolves per-class
// handling.
func TestD4AuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeD4SharedKey(true) {
		t.Errorf("authorizeD4SharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeD4SharedKey(false) {
		t.Errorf("authorizeD4SharedKey(false) = allow, want deny (family 403)")
	}
}

// TestD4AuthorizerCountersPinned pins the authorizer telemetry for
// the wave: a terminated shared-key call files one allow judgment on
// its family, and the unreachable deny branch stays observable.
func TestD4AuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v0/repositories"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	snap := tel.snapshot()
	if got := snap.authorizer["repositories"][decisionAllow]; got != 1 {
		t.Errorf("authorizer[repositories][allow] = %d, want 1", got)
	}
	if got := snap.authorizer["repositories"][decisionDeny]; got != 0 {
		t.Errorf("authorizer[repositories][deny] = %d, want 0 on the allow path", got)
	}
}

// TestD4DenyBranchShapePinned pins the deny shape for the day dest
// gains a scoped/session resolver: a non-AllScopes identity is denied
// with the documented family 403 in both negotiated shapes and the
// core is never reached. The branch is unreachable on the live routes
// today (the wave gate passes true; the live 403s are core's), so this
// pins the contract, not a live path.
func TestD4DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v0/catalog"},
		{http.MethodGet, "/api/v0/ci-cd/run-correlations"},
		{http.MethodGet, "/api/v0/ci-cd/run-correlations/count"},
		{http.MethodGet, "/api/v0/ci-cd/run-correlations/inventory"},
		{http.MethodGet, "/api/v0/cloud/resources"},
		{http.MethodGet, "/api/v0/documentation/evidence-packets/pkt_1/freshness"},
		{http.MethodGet, "/api/v0/documentation/facts"},
		{http.MethodGet, "/api/v0/documentation/findings"},
		{http.MethodGet, "/api/v0/documentation/findings/count"},
		{http.MethodGet, "/api/v0/documentation/findings/inventory"},
		{http.MethodGet, "/api/v0/documentation/findings/find_1/evidence-packet"},
		{http.MethodGet, "/api/v0/ecosystem/overview"},
		{http.MethodGet, "/api/v0/evidence/admission-decisions"},
		{http.MethodGet, "/api/v0/evidence/bundle"},
		{http.MethodGet, "/api/v0/evidence/relationships/res_1"},
		{http.MethodGet, "/api/v0/infra/resources/count"},
		{http.MethodGet, "/api/v0/infra/resources/inventory"},
		{http.MethodGet, "/api/v0/investigation-workflows"},
		{http.MethodGet, "/api/v0/investigations/deployable-unit/packet"},
		{http.MethodGet, "/api/v0/package-registry/correlations"},
		{http.MethodGet, "/api/v0/package-registry/dependencies"},
		{http.MethodGet, "/api/v0/package-registry/dependency-chains"},
		{http.MethodGet, "/api/v0/package-registry/packages"},
		{http.MethodGet, "/api/v0/package-registry/packages/count"},
		{http.MethodGet, "/api/v0/package-registry/packages/inventory"},
		{http.MethodGet, "/api/v0/package-registry/versions"},
		{http.MethodGet, "/api/v0/query-playbooks"},
		{http.MethodGet, "/api/v0/repositories"},
		{http.MethodGet, "/api/v0/repositories/by-language"},
		{http.MethodGet, "/api/v0/repositories/language-inventory"},
		{http.MethodGet, "/api/v0/repositories/repo_1/branches"},
		{http.MethodGet, "/api/v0/repositories/repo_1/content"},
		{http.MethodGet, "/api/v0/repositories/repo_1/context"},
		{http.MethodGet, "/api/v0/repositories/repo_1/coverage"},
		{http.MethodGet, "/api/v0/repositories/repo_1/freshness"},
		{http.MethodGet, "/api/v0/repositories/repo_1/stats"},
		{http.MethodGet, "/api/v0/repositories/repo_1/story"},
		{http.MethodGet, "/api/v0/repositories/repo_1/tree"},
		{http.MethodPost, "/api/v0/ecosystem/graph-summary"},
		{http.MethodPost, "/api/v0/evidence/citations"},
		{http.MethodPost, "/api/v0/infra/relationships"},
		{http.MethodPost, "/api/v0/infra/resources/search"},
		{http.MethodPost, "/api/v0/investigation-workflows/resolve"},
		{http.MethodPost, "/api/v0/query-playbooks/resolve"},
		{http.MethodPost, "/api/v0/relationships/catalog"},
		{http.MethodPost, "/api/v0/relationships/edges"},
	}
	if len(cases) != 46 {
		t.Fatalf("deny-shape cases = %d, want the 46 D4 wave routes", len(cases))
	}
	for _, tc := range cases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(tc.method, tc.path)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			srv.serveD4TerminatedAs(rec, req, false)

			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s Accept %q: status = %d, want 403", tc.method, tc.path, accept, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), d4FamilyDenyMessage) {
				t.Errorf("%s %s Accept %q: body = %q, want the family-403 message", tc.method, tc.path, accept, body)
			}
		}
	}
}

// TestD4Gated_UnauthenticatedIs401 pins that every D4 wave route keeps
// the dest 401 shape: envelope callers get the envelope error,
// everyone else the plain error_code body, both with the bare Bearer
// challenge — and the core is never contacted.
func TestD4Gated_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, entry := range d4Wave {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			key := entry.Method + " " + entry.Path
			req := httptest.NewRequest(entry.Method, d4ConcretePaths[key], nil)
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

// TestD4Gated_UnconfiguredOriginFailsClosed pins the fail-closed
// origin on the terminated path: without a valid core origin every D4
// call answers the permanent local 503 (no Retry-After) in both wire
// shapes.
func TestD4Gated_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"", "://missing-scheme-host"} {
		srv := NewServer(origin, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		for _, tc := range []struct{ method, path string }{
			{http.MethodGet, "/api/v0/repositories"},
			{http.MethodPost, "/api/v0/infra/resources/search"},
		} {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				req := authedRequest(tc.method, tc.path)
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusServiceUnavailable {
					t.Errorf("origin %q %s %s Accept %q: status = %d, want 503", origin, tc.method, tc.path, accept, rec.Code)
				}
				if got := rec.Header().Get("Retry-After"); got != "" {
					t.Errorf("origin %q %s %s Accept %q: Retry-After is set on the permanent 503", origin, tc.method, tc.path, accept)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if !strings.Contains(string(body), "backend_unavailable") {
					t.Errorf("origin %q %s %s Accept %q: body = %s, want backend_unavailable", origin, tc.method, tc.path, accept, body)
				}
			}
		}
	}
}

// TestD4Gated_TrailingSlashStaysNative pins the never-split rule: the
// trailing-slash spelling of each D4 wave route rides the wave gate
// (shared-key terminates on the native lane), not the generic relay —
// except the four d4SlashSkip literals, whose slash spellings relay
// (TestD4ConflictLiterals_TrailingSlashRelays) and are excluded here.
func TestD4Gated_TrailingSlashStaysNative(t *testing.T) {
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
		{http.MethodGet, "/api/v0/ci-cd/run-correlations/"},
		{http.MethodGet, "/api/v0/documentation/findings/"},
		{http.MethodPost, "/api/v0/ecosystem/graph-summary/"},
		{http.MethodGet, "/api/v0/package-registry/packages/"},
		{http.MethodGet, "/api/v0/repositories/"},
		{http.MethodGet, "/api/v0/repositories/repo_1/tree/"},
		{http.MethodGet, "/api/v0/evidence/bundle/"},
		{http.MethodPost, "/api/v0/evidence/citations/"},
		{http.MethodGet, "/api/v0/investigation-workflows/"},
		{http.MethodPost, "/api/v0/query-playbooks/resolve/"},
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
	for _, family := range []string{"cicd", "documentation", "infra", "package_registry", "repositories", "evidence", "investigation_workflows", "playbooks"} {
		if got := snap.dispatches[family][viaNative][outcomeCoreOK]; got < 1 {
			t.Errorf("family %q native core_ok = %d, want >= 1 (slash spelling on the native lane)", family, got)
		}
	}
}

// TestD4Gated_V1AliasTerminates pins that the v1 alias rewrite lands
// D4 wave routes on the same native handler: the core sees the
// rewritten v0 path and the response carries no deprecation headers.
func TestD4Gated_V1AliasTerminates(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range []struct{ method, v1, v0 string }{
		{http.MethodGet, "/api/v1/repositories", "/api/v0/repositories"},
		{http.MethodPost, "/api/v1/infra/resources/search", "/api/v0/infra/resources/search"},
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
