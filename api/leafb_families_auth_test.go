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

// D2 Leaf B per-family authorizer proof (Refs eshu-hq/eshu#9669, part
// of #6707). Scope-honesty label: the dest authorizer covers the
// shared-key (AllScopes) identity only — the single class dest can
// construct today. Scoped tokens, OIDC bearers, and browser sessions
// relay explicitly (core judges; TestLeafBWaveRelayExplicit... pins
// that), and the deny branch below is unreachable on live routes until
// the D8 session resolver lands. Core re-judges every forwarded call
// as backstop (tenant scope, permission catalog, governance mode),
// which is where the live 403s come from.

// TestLeafBAuthorizerSharedKeyAllowDeny pins the family gate: the
// shared-key identity allows, anything else denies. One shared
// authorizer covers all 12 gated wave routes until D8 resolves
// per-class handling.
func TestLeafBAuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeLeafBSharedKey(true) {
		t.Errorf("authorizeLeafBSharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeLeafBSharedKey(false) {
		t.Errorf("authorizeLeafBSharedKey(false) = allow, want deny (family 403)")
	}
}

// TestLeafBAuthorizerCountersPinned pins the authorizer telemetry for
// the wave: a terminated shared-key call files one allow judgment on
// its family, and the unreachable deny branch stays observable.
func TestLeafBAuthorizerCountersPinned(t *testing.T) {
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
	handler.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v0/capabilities"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	snap := tel.snapshot()
	if got := snap.authorizer["capabilities"][decisionAllow]; got != 1 {
		t.Errorf("authorizer[capabilities][allow] = %d, want 1", got)
	}
	if got := snap.authorizer["capabilities"][decisionDeny]; got != 0 {
		t.Errorf("authorizer[capabilities][deny] = %d, want 0 on the allow path", got)
	}
}

// TestLeafBDenyBranchShapePinned pins the deny shape for the day dest
// gains a scoped/session resolver: a non-AllScopes identity is denied
// with the documented family 403 in both negotiated shapes and the
// core is never reached. The branch is unreachable on the live routes
// today (the wave gate passes true; the live 403s are core's), so this
// pins the contract, not a live path.
func TestLeafBDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v0/capabilities"},
		{http.MethodPost, "/api/v0/answer-quality/score"},
		{http.MethodPost, "/api/v0/investigation/packet/render"},
		{http.MethodPost, "/api/v0/report/bundle/capture"},
		{http.MethodPost, "/api/v0/report/bundle/validate"},
		{http.MethodPost, "/api/v0/service/report/compose"},
		{http.MethodPost, "/api/v0/ask"},
		{http.MethodGet, "/api/v0/services/checkout/intelligence-report"},
		{http.MethodGet, "/api/v0/auth/admin/sign-in-policy"},
		{http.MethodGet, "/api/v0/auth/admin/provider-configs"},
		{http.MethodGet, "/api/v0/auth/admin/provider-configs/prov_1"},
		{http.MethodGet, "/api/v0/auth/admin/provider-configs/prov_1/revisions"},
	}
	if len(cases) != 12 {
		t.Fatalf("deny-shape cases = %d, want the 12 gated wave routes", len(cases))
	}
	for _, tc := range cases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(tc.method, tc.path)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			srv.serveLeafBTerminatedAs(rec, req, false)

			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s Accept %q: status = %d, want 403", tc.method, tc.path, accept, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), leafbFamilyDenyMessage) {
				t.Errorf("%s %s Accept %q: body = %q, want the family-403 message", tc.method, tc.path, accept, body)
			}
		}
	}
}

// TestLeafBGated_UnauthenticatedIs401 pins that every wave-gated
// route keeps the dest 401 shape: envelope callers get the envelope
// error, everyone else the plain error_code body, both with the bare
// Bearer challenge — and the core is never contacted.
func TestLeafBGated_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, entry := range leafbWave {
		if entry.Serve != leafbServeGated {
			continue
		}
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(entry.Method, leafbConcretePaths[entry.Method+" "+entry.Path], nil)
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

// TestLeafBGated_JunkCookieIs401WithoutCoreContact pins the
// auth-gate-overreach fix (eshu-api#51): a Cookie header with no
// session-cookie-shaped credential (a junk tracking cookie) is a
// credential-less caller — it keeps the standing dest 401 with zero
// core contact. Only a session-named cookie relays explicitly for
// core judgment; the companion relay cases live in
// TestLeafBWaveRelayExplicitForUnresolvableIdentities.
func TestLeafBGated_JunkCookieIs401WithoutCoreContact(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, cookie := range []string{
		"tracking_id=abc123",
		"eshu_session=",
		"session=Bearer " + skeletonTestKey,
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v0/capabilities", nil)
		req.Header.Set("Cookie", cookie)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Cookie %q: status = %d, want 401 (no session credential, no relay)", cookie, rec.Code)
		}
		if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Errorf("Cookie %q: WWW-Authenticate = %q, want bare Bearer", cookie, got)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (junk cookies never reach the core)", core.hits)
	}
}

// TestLeafBGated_UnconfiguredOriginFailsClosed pins the fail-closed
// origin on the terminated path: without a valid core origin every
// wave-gated call answers the permanent local 503 (no Retry-After) in
// both wire shapes.
func TestLeafBGated_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"", "://missing-scheme-host"} {
		srv := NewServer(origin, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(http.MethodGet, "/api/v0/capabilities")
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

// TestLeafBGated_TrailingSlashStaysNative pins the never-split rule:
// the trailing-slash spelling of each wave-gated route rides the wave
// gate (shared-key terminates on the native lane), not the generic
// relay.
func TestLeafBGated_TrailingSlashStaysNative(t *testing.T) {
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
		{http.MethodGet, "/api/v0/capabilities/"},
		{http.MethodPost, "/api/v0/ask/"},
		{http.MethodGet, "/api/v0/services/checkout/intelligence-report/"},
		{http.MethodGet, "/api/v0/auth/admin/provider-configs/prov_1/"},
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
	for _, family := range []string{"capabilities", "ask", "service_intel", "provider_config"} {
		if got := snap.dispatches[family][viaNative][outcomeCoreOK]; got < 1 {
			t.Errorf("family %q native core_ok = %d, want >= 1 (slash spelling on the native lane)", family, got)
		}
	}
}

// TestLeafBGated_V1AliasTerminates pins that the v1 alias rewrite lands
// wave-gated routes on the same native handler: the core sees the
// rewritten v0 path and the response carries no deprecation headers.
func TestLeafBGated_V1AliasTerminates(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v1/capabilities")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if core.lastPath != "/api/v0/capabilities" {
		t.Errorf("upstream path = %q, want the rewritten /api/v0 path", core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("Deprecation = %q, want empty on v1", got)
	}
}
