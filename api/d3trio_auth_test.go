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

// D3 Gap 02 trio per-route authorizer proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#70). Scope-honesty label: the dest
// authorizer covers the shared-key (AllScopes) identity only — the
// single class dest can construct today. Scoped tokens, OIDC bearers,
// and browser sessions relay explicitly (core judges;
// TestD3TrioWaveRelayExplicitForUnresolvableIdentities pins that, and
// TestD3TrioWaveJunkCookieStays401 pins that junk cookies never
// relay), and the deny branch below is unreachable on live routes
// until the D8 session resolver lands. Core re-judges every
// forwarded call as backstop (tenant scope, permission catalog,
// governance mode), which is where the live 403s come from. The trio
// reuses the D3 family authorizer (authorizeD3SharedKey): one shared
// trust root for the admin read family until D8 resolves per-class
// handling.

// TestD3TrioAuthorizerSharedKeyAllowDeny pins the family gate for the
// trio: the shared-key identity allows, anything else denies — the
// same authorizer the 16 D3 wave routes use.
func TestD3TrioAuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeD3SharedKey(true) {
		t.Errorf("authorizeD3SharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeD3SharedKey(false) {
		t.Errorf("authorizeD3SharedKey(false) = allow, want deny (family 403)")
	}
}

// TestD3TrioAuthorizerCountersPinned pins the authorizer telemetry
// for the trio: a terminated shared-key call files one allow judgment
// on the admin family, and the unreachable deny branch stays
// observable. One representative route per trio member.
func TestD3TrioAuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/admin/delta-union/completeness?scope=s1",
		"/api/v0/admin/reindex/progress?scope_id=s1",
		"/api/v0/admin/shared-projection/tuning-report",
	} {
		core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		tel := NewTelemetry(nil)
		srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodGet, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want 200", path, rec.Code)
		}
		snap := tel.snapshot()
		if got := snap.authorizer["admin"][decisionAllow]; got != 1 {
			t.Errorf("GET %s: authorizer[admin][allow] = %d, want 1", path, got)
		}
		if got := snap.authorizer["admin"][decisionDeny]; got != 0 {
			t.Errorf("GET %s: authorizer[admin][deny] = %d, want 0 on the allow path", path, got)
		}
	}
}

// TestD3TrioDenyBranchShapePinned pins the deny shape for the day
// dest gains a scoped/session resolver: a non-AllScopes identity is
// denied with the documented family 403 in both negotiated shapes and
// the core is never reached. The branch is unreachable on the live
// routes today (the wave gate passes true; the live 403s are core's),
// so this pins the contract, not a live path.
func TestD3TrioDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v0/admin/delta-union/completeness?scope=s1"},
		{http.MethodGet, "/api/v0/admin/reindex/progress?scope_id=s1"},
		{http.MethodGet, "/api/v0/admin/shared-projection/tuning-report"},
	}
	if len(cases) != len(d3TrioWave) {
		t.Fatalf("deny-shape cases = %d, want the %d trio routes", len(cases), len(d3TrioWave))
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

// TestD3TrioGated_UnauthenticatedIs401 pins that every trio route
// keeps the dest 401 shape: envelope callers get the envelope error,
// everyone else the plain error_code body, both with the bare Bearer
// challenge — and the core is never contacted.
func TestD3TrioGated_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, entry := range d3TrioWave {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			key := entry.Method + " " + entry.Path
			req := httptest.NewRequest(entry.Method, d3TrioConcretePaths[key], nil)
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
