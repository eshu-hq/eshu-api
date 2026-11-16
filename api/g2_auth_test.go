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

// G2 per-family authorizer proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#101 RE:G2V2). Scope-honesty label: the
// dest authorizer covers the shared-key (AllScopes) identity only —
// the single class dest can construct today. Scoped tokens, OIDC
// bearers, and browser sessions relay explicitly (core judges;
// TestG2WaveRelayExplicit... pins that), and the deny branch below
// is unreachable on live routes until the D8 session resolver lands.
// Core re-judges every forwarded call as backstop (tenant scope,
// permission catalog, governance mode), which is where the live
// 403s come from. Every leg classifies to the shared "other"
// family, so the counters pin that value.

// TestG2AuthorizerSharedKeyAllowDeny pins the family gate: the
// shared-key identity allows, anything else denies. One shared
// authorizer covers all 7 G2 routes until D8 resolves per-class
// handling.
func TestG2AuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeG2SharedKey(true) {
		t.Errorf("authorizeG2SharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeG2SharedKey(false) {
		t.Errorf("authorizeG2SharedKey(false) = allow, want deny (family 403)")
	}
}

// TestG2AuthorizerCountersPinned pins the authorizer telemetry for
// the wave: a terminated shared-key call files one allow judgment
// on the route family, and the unreachable deny branch stays
// observable. One leg per wave route.
func TestG2AuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for key, target := range g2ConcretePaths {
		entry := g2WaveEntry(key)
		var req *http.Request
		if entry.Method == http.MethodPost {
			req = g2AuthedReq(entry.Method, target, `{"packet":"derive-probe"}`)
		} else {
			req = g2AuthedReq(entry.Method, target, "")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: status = %d, want 200", entry.Method, target, rec.Code)
		}
	}
	snap := tel.snapshot()
	if got := snap.authorizer["other"][decisionAllow]; got != uint64(len(g2Wave)) {
		t.Errorf("authorizer[other][allow] = %d, want %d (one per wave leg)", got, len(g2Wave))
	}
	if got := snap.authorizer["other"][decisionDeny]; got != 0 {
		t.Errorf("authorizer[other][deny] = %d, want 0 on the allow path", got)
	}
}

// TestG2DenyBranchShapePinned pins the deny shape for the day dest
// gains a scoped/session resolver: a non-AllScopes identity is
// denied with the documented family 403 in both negotiated shapes
// and the core is never reached. The branch is unreachable on the
// live routes today (the wave gate passes true; the live 403s are
// core's), so this pins the contract, not a live path.
func TestG2DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for key, target := range g2ConcretePaths {
		entry := g2WaveEntry(key)
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := entry.Method + " " + target + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				var req *http.Request
				if entry.Method == http.MethodPost {
					req = g2AuthedReq(entry.Method, target, `{"packet":"derive-probe"}`)
				} else {
					req = g2AuthedReq(entry.Method, target, "")
				}
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				srv.serveG2TerminatedAs(rec, req, false)

				res := rec.Result()
				if res.StatusCode != http.StatusForbidden {
					t.Fatalf("status = %d, want the documented 403", res.StatusCode)
				}
				body, _ := io.ReadAll(res.Body)
				if !strings.Contains(string(body), g2FamilyDenyMessage) {
					t.Errorf("body = %q, want it to carry the documented deny text", body)
				}
			})
		}
	}
}

// TestG2WaveRelayExplicitForUnresolvableIdentities pins the
// never-invent rule for the whole wave: every G2 route carrying a
// foreign credential class relays explicitly on the relay lane with
// core bytes untouched.
func TestG2WaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, entry := range g2Wave {
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
				target, ok := g2ConcretePaths[key]
				if !ok {
					t.Fatalf("no concrete path for wave route %q", key)
				}
				body := ""
				if entry.Method == http.MethodPost {
					body = `{"packet":"derive-probe"}`
				}
				req := g2UnauthedReq(entry.Method, target, body)
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
				gotBody, _ := io.ReadAll(rec.Result().Body)
				if string(gotBody) != core.body {
					t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, gotBody, core.body)
				}
				snap := tel.snapshot()
				family := classifyFamily(target)
				lanes := snap.dispatches[family]
				if lanes[viaRelay] == nil {
					t.Errorf("%s: family %q dispatches = %v, want a relay lane row (never invented)", tc.name, family, lanes)
				}
			}
		})
	}
}

// TestG2WaveJunkCookieStays401 pins the D2 follow-up lesson for the
// wave: a Cookie header without a session-cookie-shaped member is
// junk, not a browser session, and stays dest-401 with zero core
// contact.
func TestG2WaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	for key, target := range g2ConcretePaths {
		entry := g2WaveEntry(key)
		t.Run(entry.Method+" "+target, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			body := ""
			if entry.Method == http.MethodPost {
				body = `{"packet":"derive-probe"}`
			}
			req := g2UnauthedReq(entry.Method, target, body)
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

// TestG2WaveCredentiallessStays401 pins that a credential-less wave
// call gets the standing dest 401 with zero core contact.
func TestG2WaveCredentiallessStays401(t *testing.T) {
	t.Parallel()

	for key, target := range g2ConcretePaths {
		entry := g2WaveEntry(key)
		t.Run(entry.Method+" "+target, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			body := ""
			if entry.Method == http.MethodPost {
				body = `{"packet":"derive-probe"}`
			}
			req := g2UnauthedReq(entry.Method, target, body)
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
