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

// Compare relay-leg per-family authorizer and wave-gate proof (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#72, gap 04).
// Scope-honesty label: the dest authorizer covers the shared-key
// (AllScopes) identity only — the single class dest can construct
// today. Scoped tokens, OIDC bearers, and browser sessions relay
// explicitly (core judges; TestCompareWaveRelayExplicit... pins
// that), and the deny branch below is unreachable on the live route
// until the D8 session resolver lands. Core re-judges every
// forwarded call as backstop (tenant scope, permission catalog,
// governance mode), which is where the live 403s come from. The
// route stays authed: no public carve-out exists in this lane, and
// keyless callers keep the standing dest 401 with zero core contact.

// TestCompareAuthorizerSharedKeyAllowDeny pins the family gate: the
// shared-key identity allows, anything else denies. One shared
// authorizer covers the Compare wave route until D8 resolves
// per-class handling.
func TestCompareAuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeCompareSharedKey(true) {
		t.Errorf("authorizeCompareSharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeCompareSharedKey(false) {
		t.Errorf("authorizeCompareSharedKey(false) = allow, want deny (family 403)")
	}
}

// TestCompareAuthorizerCountersPinned pins the authorizer telemetry
// for the wave: a terminated shared-key call files one allow
// judgment on the compare family, and the unreachable deny branch
// stays observable.
func TestCompareAuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"workload":"w1"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, compareAuthedPost("application/json"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	snap := tel.snapshot()
	if got := snap.authorizer["compare"][decisionAllow]; got != 1 {
		t.Errorf("authorizer[compare][allow] = %d, want 1", got)
	}
	if got := snap.authorizer["compare"][decisionDeny]; got != 0 {
		t.Errorf("authorizer[compare][deny] = %d, want 0 on the allow path", got)
	}
}

// TestCompareDenyBranchShapePinned pins the deny shape for the day
// dest gains a scoped/session resolver: a non-AllScopes identity is
// denied with the documented family 403 in both negotiated shapes
// and the core is never reached. The branch is unreachable on the
// live route today (the wave gate passes true; the live 403s are
// core's), so this pins the contract, not a live path.
func TestCompareDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		name := "POST " + compareConcretePath + " " + accept
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := compareAuthedPost(accept)
			rec := httptest.NewRecorder()
			srv.serveCompareTerminatedAs(rec, req, false)

			res := rec.Result()
			if res.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want the documented 403", res.StatusCode)
			}
			body, _ := io.ReadAll(res.Body)
			if !strings.Contains(string(body), compareFamilyDenyMessage) {
				t.Errorf("body = %q, want it to carry the documented deny text", body)
			}
		})
	}
}

// TestCompareWaveRelayExplicitForUnresolvableIdentities pins the
// D-wave ruling's never-invent rule for the wave: the Compare POST
// carrying a credential class dest cannot resolve (a scoped token,
// an OIDC bearer, a browser session cookie) must relay explicitly to
// the core — the core judges, core bytes come back untouched — and
// must never answer a dest-invented 401, 403, 404, or 500.
func TestCompareWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		header map[string]string
	}{
		{"scoped bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
		{"oidc bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
		{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
		{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{
				status: http.StatusOK,
				body:   `{"workload":"w1","story":"same"}`,
			}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, compareConcretePath, strings.NewReader(compareParityBody))
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Errorf("%s: upstream hits = %d, want 1 (relay explicitly, never gate-invent)", tc.name, core.hits)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("%s: status = %d, want the relayed core 200 (never 401/403/404/500-invented)", tc.name, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != core.body {
				t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, body, core.body)
			}
			if core.lastBody != compareParityBody {
				t.Errorf("%s: upstream body = %q, want the compare request %q forwarded intact", tc.name, core.lastBody, compareParityBody)
			}
			snap := tel.snapshot()
			lanes := snap.dispatches["compare"]
			if lanes[viaRelay] == nil {
				t.Errorf("%s: family %q dispatches = %v, want a relay lane row (never invented)", tc.name, "compare", lanes)
			}
		})
	}
}

// TestCompareWaveJunkCookieStays401 pins the D2 follow-up lesson for
// the wave: a Cookie header without a session-cookie-shaped member
// is junk, not a browser session, and stays dest-401 with zero core
// contact.
func TestCompareWaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"workload":"w1"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodPost, compareConcretePath, strings.NewReader(compareParityBody))
	req.Header.Set("Cookie", "junk=1; other=abc")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want the standing dest 401", rec.Code)
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (junk cookie is not a credential)", core.hits)
	}
}

// TestCompareWaveCredentiallessStays401 pins that a
// credential-less Compare call gets the standing dest 401 with zero
// core contact: the route stays authed, with no public carve-out in
// this lane.
func TestCompareWaveCredentiallessStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"workload":"w1"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodPost, compareConcretePath, strings.NewReader(compareParityBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want the standing dest 401", rec.Code)
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (no credential, no core contact)", core.hits)
	}
}
