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

// Wave-2 Content per-family authorizer proof (Refs
// eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491, satellite
// eshu-hq/eshu-api#72). Scope-honesty label: the dest authorizer
// covers the shared-key (AllScopes) identity only — the single class
// dest can construct today. Scoped tokens, OIDC bearers, and browser
// sessions relay explicitly (core judges;
// TestContentPointReadsWaveRelayExplicit... pins that), and the deny
// branch below is unreachable on live routes until the D8 session
// resolver lands. Core re-judges every forwarded call as backstop
// (tenant scope, permission catalog, governance mode), which is where
// the live 403s come from.

// TestContentPointReadsAuthorizerSharedKeyAllowDeny pins the family
// gate: the shared-key identity allows, anything else denies. One
// shared authorizer covers all 3 wave routes until D8 resolves
// per-class handling.
func TestContentPointReadsAuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeContentSharedKey(true) {
		t.Errorf("authorizeContentSharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeContentSharedKey(false) {
		t.Errorf("authorizeContentSharedKey(false) = allow, want deny (family 403)")
	}
}

// TestContentPointReadsAuthorizerCountersPinned pins the authorizer
// telemetry for the wave: terminated shared-key calls file one allow
// judgment each on the content family, and the unreachable deny branch
// stays observable.
func TestContentPointReadsAuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"repo_id":"repo-1"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	targets := []struct {
		path string
		body string
	}{
		{contentFilesReadPath, `{"repo_id":"repo-1","relative_path":"src/app.go"}`},
		{contentFilesLinesPath, `{"repo_id":"repo-1","relative_path":"src/app.go","start_line":1,"end_line":2}`},
		{contentEntitiesReadPath, `{"entity_id":"entity-1"}`},
	}
	for _, target := range targets {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, contentAuthedPost(target.path, target.body, "application/json"))
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s: status = %d, want 200", target.path, rec.Code)
		}
	}
	snap := tel.snapshot()
	if got := snap.authorizer["content"][decisionAllow]; got != 3 {
		t.Errorf("authorizer[content][allow] = %d, want 3 (one per wave leg)", got)
	}
	if got := snap.authorizer["content"][decisionDeny]; got != 0 {
		t.Errorf("authorizer[content][deny] = %d, want 0 on the allow path", got)
	}
}

// TestContentPointReadsDenyBranchShapePinned pins the deny shape for
// the day dest gains a scoped/session resolver: a non-AllScopes
// identity is denied with the documented family 403 in both negotiated
// shapes and the core is never reached. The branch is unreachable on
// the live routes today (the wave gate passes true; the live 403s are
// core's), so this pins the contract, not a live path.
func TestContentPointReadsDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	cases := []struct {
		path string
		body string
	}{
		{contentFilesReadPath, `{"repo_id":"repo-1","relative_path":"src/app.go"}`},
		{contentFilesLinesPath, `{"repo_id":"repo-1","relative_path":"src/app.go","start_line":1,"end_line":2}`},
		{contentEntitiesReadPath, `{"entity_id":"entity-1"}`},
	}
	for _, tc := range cases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := "POST " + tc.path + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				req := contentAuthedPost(tc.path, tc.body, accept)
				rec := httptest.NewRecorder()
				srv.serveContentTerminatedAs(rec, req, false)

				res := rec.Result()
				if res.StatusCode != http.StatusForbidden {
					t.Fatalf("status = %d, want the documented 403", res.StatusCode)
				}
				body, _ := io.ReadAll(res.Body)
				if !strings.Contains(string(body), contentFamilyDenyMessage) {
					t.Errorf("body = %q, want it to carry the documented deny text", body)
				}
			})
		}
	}
}

// TestContentPointReadsWaveRelayExplicitForUnresolvableIdentities pins
// the never-invent rule for the whole wave: every wave route carrying
// a foreign credential class relays explicitly on the relay lane with
// core bytes untouched.
func TestContentPointReadsWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, entry := range contentWave {
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
					body:   `{"repo_id":"repo-1"}`,
				}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				key := entry.Method + " " + entry.Path
				target, ok := contentConcretePaths[key]
				if !ok {
					t.Fatalf("no concrete path for wave route %q", key)
				}
				body := contentReadBodies[key][0]
				req := httptest.NewRequest(entry.Method, target, strings.NewReader(body))
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
				respBody, _ := io.ReadAll(rec.Result().Body)
				if string(respBody) != core.body {
					t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, respBody, core.body)
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

// TestContentPointReadsWaveJunkCookieStays401 pins the D2 follow-up
// lesson for the wave: a Cookie header without a session-cookie-shaped
// member is junk, not a browser session, and stays dest-401 with zero
// core contact.
func TestContentPointReadsWaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	targets := []string{contentFilesReadPath, contentFilesLinesPath, contentEntitiesReadPath}
	for _, target := range targets {
		t.Run("POST "+target, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"repo_id":"repo-1"}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{}`))
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

// TestContentPointReadsWaveCredentiallessStays401 pins that a
// credential-less wave call gets the standing dest 401 with zero core
// contact.
func TestContentPointReadsWaveCredentiallessStays401(t *testing.T) {
	t.Parallel()

	targets := []string{contentFilesReadPath, contentFilesLinesPath, contentEntitiesReadPath}
	for _, target := range targets {
		t.Run("POST "+target, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"repo_id":"repo-1"}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{}`))
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
