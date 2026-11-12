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

// Code-flow relay-leg per-family authorizer and wave-gate proof (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#98, W6a).
// Scope-honesty label: the dest authorizer covers the shared-key
// (AllScopes) identity only — the single class dest can construct
// today. Scoped tokens, OIDC bearers, and browser sessions relay
// explicitly (core judges; TestCodeFlowWaveRelayExplicit... pins
// that), and the deny branch below is unreachable on the live route
// until the D8 session resolver lands. Core re-judges every
// forwarded call as backstop, which is where the live 403s come from.
// The routes stay authed: no public carve-out exists in this lane,
// and keyless callers keep the standing dest 401 with zero core
// contact.

// TestCodeFlowAuthorizerSharedKeyAllowDeny pins the family gate: the
// shared-key identity allows, anything else denies. One shared
// authorizer covers the four Code-flow wave routes until D8 resolves
// per-class handling.
func TestCodeFlowAuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeCodeFlowSharedKey(true) {
		t.Errorf("authorizeCodeFlowSharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeCodeFlowSharedKey(false) {
		t.Errorf("authorizeCodeFlowSharedKey(false) = allow, want deny (family 403)")
	}
}

// TestCodeFlowAuthorizerCountersPinned pins the authorizer telemetry
// for the wave: a terminated shared-key call files one allow
// judgment on the code_flow family, and the unreachable deny branch
// stays observable.
func TestCodeFlowAuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	for _, path := range codeFlowConcretePaths {
		t.Run("POST "+path, func(t *testing.T) {
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
			handler.ServeHTTP(rec, codeFlowAuthedPost(path, "application/json"))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.authorizer["code_flow"][decisionAllow]; got != 1 {
				t.Errorf("authorizer[code_flow][allow] = %d, want 1", got)
			}
			if got := snap.authorizer["code_flow"][decisionDeny]; got != 0 {
				t.Errorf("authorizer[code_flow][deny] = %d, want 0 on the allow path", got)
			}
		})
	}
}

// TestCodeFlowDenyBranchShapePinned pins the deny shape for the day
// dest gains a scoped/session resolver: a non-AllScopes identity is
// denied with the documented family 403 in both negotiated shapes
// and the core is never reached.
func TestCodeFlowDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, path := range codeFlowConcretePaths {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := "POST " + path + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				req := codeFlowAuthedPost(path, accept)
				rec := httptest.NewRecorder()
				srv.serveCodeFlowTerminatedAs(rec, req, false)

				res := rec.Result()
				if res.StatusCode != http.StatusForbidden {
					t.Fatalf("status = %d, want the documented 403", res.StatusCode)
				}
				body, _ := io.ReadAll(res.Body)
				if !strings.Contains(string(body), codeFlowFamilyDenyMessage) {
					t.Errorf("body = %q, want it to carry the documented deny text", body)
				}
			})
		}
	}
}

// TestCodeFlowWaveRelayExplicitForUnresolvableIdentities pins the
// W-wave ruling's never-invent rule for the wave: each Code-flow POST
// carrying a credential class dest cannot resolve (a scoped token,
// an OIDC bearer, a browser session cookie) must relay explicitly to
// the core — the core judges, core bytes come back untouched — and
// must never answer a dest-invented 401, 403, 404, or 500.
func TestCodeFlowWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, path := range codeFlowConcretePaths {
		for _, tc := range []struct {
			name   string
			header map[string]string
		}{
			{"scoped bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
			{"oidc bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
			{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
			{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
		} {
			t.Run("POST "+path+" "+tc.name, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{
					status: http.StatusOK,
					body:   `{"data":[]}`,
				}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(codeFlowParityBody))
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
				if core.lastBody != codeFlowParityBody {
					t.Errorf("%s: upstream body = %q, want the code-flow request %q forwarded intact", tc.name, core.lastBody, codeFlowParityBody)
				}
				snap := tel.snapshot()
				lanes := snap.dispatches["code_flow"]
				if lanes[viaRelay] == nil {
					t.Errorf("%s: family %q dispatches = %v, want a relay lane row (never invented)", tc.name, "code_flow", lanes)
				}
			})
		}
	}
}

// TestCodeFlowWaveGateEmptyKey401 pins the fail-closed gate: with no
// configured key every wave call answers the standing dest 401 with
// zero core contact, for the bare and slash spellings.
func TestCodeFlowWaveGateEmptyKey401(t *testing.T) {
	t.Parallel()

	for _, path := range codeFlowConcretePaths {
		for _, target := range []string{path, path + "/"} {
			t.Run("POST "+target, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, "")
				handler := srv.Handler(DefaultV0Sunset)

				req := codeFlowAuthedPost(target, "application/json")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusUnauthorized {
					t.Errorf("status = %d, want the standing dest 401", rec.Code)
				}
				if core.hits != 0 {
					t.Errorf("upstream hits = %d, want 0 (gate denies with zero core contact)", core.hits)
				}
			})
		}
	}
}
