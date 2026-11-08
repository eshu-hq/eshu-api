// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// D6 Status + SupplyChain relay-explicit and credential-shape proof
// (Refs eshu-hq/eshu#9669, part of #6707). The D-wave ruling's
// never-invent rule: a wave-gated route carrying a credential class
// dest cannot resolve (a scoped token, an OIDC bearer, a browser
// session cookie) must relay explicitly to the core — the core judges,
// core bytes come back untouched — and must never answer a
// dest-invented 401, 403, 404, or 500. Junk cookies stay dest-401 with
// zero core contact.

// TestD6WaveRelayExplicitForUnresolvableIdentities pins the
// never-invent rule for the whole wave: every D6 route carrying a
// foreign credential class relays explicitly on the relay lane with
// core bytes untouched.
func TestD6WaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, entry := range d6Wave {
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
				req := httptest.NewRequest(entry.Method, d6ConcretePaths[key], nil)
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
				family := classifyFamily(d6ConcretePaths[key])
				lanes := snap.dispatches[family]
				if lanes[viaRelay] == nil {
					t.Errorf("%s: family %q dispatches = %v, want a relay lane row (never invented)", tc.name, family, lanes)
				}
			}
		})
	}
}

// TestD6WaveJunkCookieStays401 pins the D2 follow-up lesson for the
// wave: a Cookie header without a session-cookie-shaped member is junk,
// not a browser session, and stays dest-401 with zero core contact.
func TestD6WaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	targets := []string{
		"/api/v0/status/pipeline",
		"/api/v0/status/collectors",
		"/api/v0/supply-chain/impact/findings",
		"/api/v0/supply-chain/security-alerts/reconciliations",
	}
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

// TestD6WaveCredentiallessStays401 pins that a credential-less wave
// call gets the standing dest 401 with zero core contact.
func TestD6WaveCredentiallessStays401(t *testing.T) {
	t.Parallel()

	targets := []string{
		"/api/v0/collector-readiness",
		"/api/v0/supply-chain/advisories",
	}
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

// TestD6RelayedRoutesCarryRelayLane pins that the disclosed relay-only
// routes file their dispatch on the relay lane, never the native lane:
// the scoping line is observable from dest signals.
func TestD6RelayedRoutesCarryRelayLane(t *testing.T) {
	t.Parallel()

	relayed := []string{
		"/api/v0/status/governance",
		"/api/v0/status/answer-narration",
		"/api/v0/status/hosted-readiness",
		"/api/v0/investigations/supply-chain/impact/packet",
	}
	for _, target := range relayed {
		t.Run("GET "+target, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, authedRequest(http.MethodGet, target))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(target)
			lanes := snap.dispatches[family]
			if lanes[viaNative] != nil {
				t.Errorf("family %q dispatches = %v, want no native lane row for the relayed route", family, lanes)
			}
		})
	}
}

// TestD6SlashSpellingsTerminateNatively pins the never-split rule for
// the wave: every trailing-slash spelling terminates on the native
// lane exactly like its bare spelling.
func TestD6SlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for key, concrete := range d6ConcretePaths {
		t.Run(key+" slash", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, authedRequest(http.MethodGet, concrete+"/"))

			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s/: status = %d, want 200 from the stub core", concrete, rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(concrete)
			if got := snap.dispatches[family][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", family, got)
			}
		})
	}
}
