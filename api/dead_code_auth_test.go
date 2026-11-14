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

// Dead-code per-family authorizer and wave-gate proof (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#124).
// Scope-honesty label: the dest authorizer covers the shared-key
// (AllScopes) identity only — the single class dest can construct
// today. Scoped tokens, OIDC bearers, and browser sessions relay
// explicitly (core judges;
// TestDeadCodeWaveRelayExplicit... pins that), and the deny branch
// below is unreachable on the live route until the D8 session resolver
// lands. Core re-judges every forwarded call as backstop, which is
// where the live 403s come from.
// The routes stay authed: no public carve-out exists in this lane,
// and keyless callers keep the standing dest 401 with zero core
// contact.

// TestDeadCodeAuthorizerSharedKeyAllowDeny pins the family gate: the
// shared-key identity allows, anything else denies. One shared
// authorizer covers all three dead-code wave routes until D8 resolves
// per-class handling.
func TestDeadCodeAuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeDeadCodeSharedKey(true) {
		t.Errorf("authorizeDeadCodeSharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeDeadCodeSharedKey(false) {
		t.Errorf("authorizeDeadCodeSharedKey(false) = allow, want deny (family 403)")
	}
}

// TestDeadCodeAuthorizerCountersPinned pins the authorizer telemetry
// for the wave: a terminated shared-key call files one allow judgment
// on the code_dead_code family, and the unreachable deny branch stays
// observable.
func TestDeadCodeAuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	for _, leg := range []struct {
		path string
		body string
	}{
		{deadCodeScanPath, deadCodeScanBodies[0]},
		{deadCodeCrossRepoPath, deadCodeCrossRepoBodies[0]},
		{deadCodeInvestigatePath, deadCodeInvestigateBodies[0]},
	} {
		t.Run("POST "+leg.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"candidates":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, deadCodeAuthedPost(leg.path, leg.body, "application/json"))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.authorizer[deadCodeFamily][decisionAllow]; got != 1 {
				t.Errorf("authorizer[%s][allow] = %d, want 1", deadCodeFamily, got)
			}
			if got := snap.authorizer[deadCodeFamily][decisionDeny]; got != 0 {
				t.Errorf("authorizer[%s][deny] = %d, want 0 on the allow path", deadCodeFamily, got)
			}
		})
	}
}

// TestDeadCodeDenyBranchShapePinned pins the deny shape for the day
// dest gains a scoped/session resolver: a non-AllScopes identity is
// denied with the documented family 403 in both negotiated shapes
// and the core is never reached — for every wave route.
func TestDeadCodeDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	legs := []struct {
		name string
		path string
		body string
	}{
		{"POST scan", deadCodeScanPath, deadCodeScanBodies[0]},
		{"POST cross-repo", deadCodeCrossRepoPath, deadCodeCrossRepoBodies[0]},
		{"POST investigate", deadCodeInvestigatePath, deadCodeInvestigateBodies[0]},
	}
	for _, leg := range legs {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.name + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				req := deadCodeAuthedPost(leg.path, leg.body, accept)
				rec := httptest.NewRecorder()
				srv.serveDeadCodeTerminatedAs(rec, req, false)

				res := rec.Result()
				if res.StatusCode != http.StatusForbidden {
					t.Fatalf("status = %d, want the documented 403", res.StatusCode)
				}
				body, _ := io.ReadAll(res.Body)
				if !strings.Contains(string(body), deadCodeFamilyDenyMessage) {
					t.Errorf("body = %q, want it to carry the documented deny text", body)
				}
			})
		}
	}
}

// TestDeadCodeWaveRelayExplicitForUnresolvableIdentities pins the
// gated forward-model never-invent rule for the wave: each dead-code
// route carrying a credential class dest cannot resolve (a scoped token,
// an OIDC bearer, a browser session cookie) must relay explicitly to the
// core — the core judges, core bytes come back untouched — and must
// never answer a dest-invented 401, 403, 404, 409, 422, or 500.
func TestDeadCodeWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	legs := []struct {
		name string
		path string
		body string
	}{
		{"POST scan", deadCodeScanPath, deadCodeScanBodies[0]},
		{"POST cross-repo", deadCodeCrossRepoPath, deadCodeCrossRepoBodies[0]},
		{"POST investigate", deadCodeInvestigatePath, deadCodeInvestigateBodies[0]},
	}
	for _, leg := range legs {
		for _, tc := range []struct {
			name   string
			header map[string]string
		}{
			{"scoped bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
			{"oidc bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
			{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
			{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
		} {
			t.Run(leg.name+" "+tc.name, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{
					status: http.StatusOK,
					body:   `{"candidates":[]}`,
				}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				req := httptest.NewRequest(http.MethodPost, leg.path, strings.NewReader(leg.body))
				for k, v := range tc.header {
					req.Header.Set(k, v)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Errorf("%s: upstream hits = %d, want 1 (relay explicitly, never gate-invent)", tc.name, core.hits)
				}
				if rec.Code != http.StatusOK {
					t.Errorf("%s: status = %d, want the relayed core 200 (never 401/403/404/409/422/500-invented)", tc.name, rec.Code)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != core.body {
					t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, body, core.body)
				}
				snap := tel.snapshot()
				lanes, ok := snap.dispatches["code_dead_code"]
				if !ok || lanes[viaRelay] == nil {
					t.Errorf("%s: dispatches = %v, want a relay lane row on exactly the code_dead_code family (never invented, never misfiled)", tc.name, snap.dispatches)
				}
			})
		}
	}
}

// TestDeadCodeWaveGateEmptyKey401 pins the fail-closed gate: with no
// configured key every wave call answers the standing dest 401 with
// zero core contact, for the bare and slash spellings of every route.
func TestDeadCodeWaveGateEmptyKey401(t *testing.T) {
	t.Parallel()

	targets := []struct {
		path string
		body string
	}{
		{deadCodeScanPath, deadCodeScanBodies[0]},
		{deadCodeCrossRepoPath, deadCodeCrossRepoBodies[0]},
		{deadCodeInvestigatePath, deadCodeInvestigateBodies[0]},
	}
	for _, target := range targets {
		for _, slash := range []string{"", "/"} {
			t.Run("POST "+target.path+slash, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"candidates":[]}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, "")
				handler := srv.Handler(DefaultV0Sunset)

				req := deadCodeAuthedPost(target.path, target.body, "application/json")
				req.URL.Path = target.path + slash
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

// TestDeadCodeWaveJunkCookieStays401 pins the D2 follow-up lesson
// for the wave: a Cookie header with no session-cookie member is not
// a credential and answers the standing dest 401 with zero core
// contact.
func TestDeadCodeWaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	for _, leg := range []struct {
		path string
		body string
	}{
		{deadCodeScanPath, deadCodeScanBodies[0]},
		{deadCodeCrossRepoPath, deadCodeCrossRepoBodies[0]},
	} {
		t.Run("POST "+leg.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"candidates":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, leg.path, strings.NewReader(leg.body))
			req.Header.Set("Cookie", "theme=dark")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing dest 401 for a junk cookie", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (junk cookie never reaches the core)", core.hits)
			}
		})
	}
}

// TestDeadCodeWaveCredentiallessStays401 pins that a caller with no
// credential at all answers the standing dest 401 with zero core
// contact on every wave route.
func TestDeadCodeWaveCredentiallessStays401(t *testing.T) {
	t.Parallel()

	for _, leg := range []struct {
		path string
		body string
	}{
		{deadCodeScanPath, deadCodeScanBodies[0]},
		{deadCodeCrossRepoPath, deadCodeCrossRepoBodies[0]},
		{deadCodeInvestigatePath, deadCodeInvestigateBodies[0]},
	} {
		t.Run("POST "+leg.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"candidates":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, leg.path, strings.NewReader(leg.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing dest 401", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (credential-less callers never reach the core)", core.hits)
			}
		})
	}
}
