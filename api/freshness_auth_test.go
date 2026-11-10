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

// Wave-1 Freshness per-family authorizer proof (Refs
// eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491, satellite
// eshu-hq/eshu-api#72). Scope-honesty label: the dest authorizer
// covers the shared-key (AllScopes) identity only — the single class
// dest can construct today. Scoped tokens, OIDC bearers, and browser
// sessions relay explicitly (core judges;
// TestFreshnessWaveRelayExplicit... pins that), and the deny branch
// below is unreachable on live routes until the D8 session resolver
// lands. Core re-judges every forwarded call as backstop (tenant
// scope, permission catalog, governance mode), which is where the live
// 403s come from.

// TestFreshnessAuthorizerSharedKeyAllowDeny pins the family gate: the
// shared-key identity allows, anything else denies. One shared
// authorizer covers all 3 Wave-1 routes until D8 resolves per-class
// handling.
func TestFreshnessAuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeFreshnessSharedKey(true) {
		t.Errorf("authorizeFreshnessSharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeFreshnessSharedKey(false) {
		t.Errorf("authorizeFreshnessSharedKey(false) = allow, want deny (family 403)")
	}
}

// TestFreshnessAuthorizerCountersPinned pins the authorizer telemetry
// for the wave: a terminated shared-key call files one allow judgment
// on the freshness family, and the unreachable deny branch stays
// observable.
func TestFreshnessAuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{freshnessChangedSincePath, freshnessGenerationsPath, freshnessServiceChangedSincePath} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest(http.MethodGet, target))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want 200", target, rec.Code)
		}
	}
	snap := tel.snapshot()
	if got := snap.authorizer["freshness"][decisionAllow]; got != 3 {
		t.Errorf("authorizer[freshness][allow] = %d, want 3 (one per wave leg)", got)
	}
	if got := snap.authorizer["freshness"][decisionDeny]; got != 0 {
		t.Errorf("authorizer[freshness][deny] = %d, want 0 on the allow path", got)
	}
}

// TestFreshnessDenyBranchShapePinned pins the deny shape for the day
// dest gains a scoped/session resolver: a non-AllScopes identity is
// denied with the documented family 403 in both negotiated shapes and
// the core is never reached. The branch is unreachable on the live
// routes today (the wave gate passes true; the live 403s are core's),
// so this pins the contract, not a live path.
func TestFreshnessDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, freshnessChangedSincePath},
		{http.MethodGet, freshnessGenerationsPath},
		{http.MethodGet, freshnessServiceChangedSincePath},
	}
	for _, tc := range cases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := tc.method + " " + tc.path + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				req := authedRequest(tc.method, tc.path)
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				srv.serveFreshnessTerminatedAs(rec, req, false)

				res := rec.Result()
				if res.StatusCode != http.StatusForbidden {
					t.Fatalf("status = %d, want the documented 403", res.StatusCode)
				}
				body, _ := io.ReadAll(res.Body)
				if !strings.Contains(string(body), freshnessFamilyDenyMessage) {
					t.Errorf("body = %q, want it to carry the documented deny text", body)
				}
			})
		}
	}
}

// TestFreshnessWaveRelayExplicitForUnresolvableIdentities pins the
// never-invent rule for the whole wave: every Wave-1 route carrying a
// foreign credential class relays explicitly on the relay lane with
// core bytes untouched.
func TestFreshnessWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, entry := range freshnessWave {
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
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
				core := &stubCore{
					status: http.StatusOK,
					body:   `{"data":[],"error":null,"truth":null}`,
				}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				key := entry.Method + " " + entry.Path
				req := httptest.NewRequest(entry.Method, freshnessConcretePaths[key], nil)
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
				snap := tel.snapshot()
				family := classifyFamily(freshnessConcretePaths[key])
				lanes := snap.dispatches[family]
				if lanes[viaRelay] == nil {
					t.Errorf("%s: family %q dispatches = %v, want a relay lane row (never invented)", tc.name, family, lanes)
				}
			}
		})
	}
}

// TestFreshnessWaveJunkCookieStays401 pins the D2 follow-up lesson for
// the wave: a Cookie header without a session-cookie-shaped member is
// junk, not a browser session, and stays dest-401 with zero core
// contact.
func TestFreshnessWaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	targets := []string{freshnessChangedSincePath, freshnessGenerationsPath, freshnessServiceChangedSincePath}
	for _, target := range targets {
		t.Run("GET "+target, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, target, nil)
			req.Header.Set("Cookie", "junk=1; other=abc")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing dest 401", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (junk cookie is not a credential)", core.hits)
			}
		})
	}
}

// TestFreshnessWaveCredentiallessStays401 pins that a credential-less
// wave call gets the standing dest 401 with zero core contact.
func TestFreshnessWaveCredentiallessStays401(t *testing.T) {
	t.Parallel()

	targets := []string{freshnessChangedSincePath, freshnessGenerationsPath, freshnessServiceChangedSincePath}
	for _, target := range targets {
		t.Run("GET "+target, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing dest 401", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (no credential, no core contact)", core.hits)
			}
		})
	}
}
