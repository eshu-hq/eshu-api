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

// W8a satellite termination auth proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#100). The D-wave ruling's never-invent
// rule: a wave-gated route carrying a credential class dest cannot
// resolve (a scoped token, an OIDC bearer, a browser session cookie)
// must relay explicitly to the core — the core judges, core bytes
// come back untouched — and must never answer a dest-invented 401,
// 403, 404, or 500. Junk cookies stay dest-401 with zero core
// contact.

// TestW8aWaveRelayExplicitForUnresolvableIdentities pins the
// never-invent rule for the whole wave: every W8a route carrying a
// foreign credential class relays explicitly on the relay lane with
// core bytes untouched.
func TestW8aWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, entry := range w8aWave {
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

				// Foreign-credential callers carry ONLY the foreign
				// class: bundling the valid shared key alongside
				// would terminate as shared-key, which is the
				// gate's documented precedence, not an invention.
				var req *http.Request
				if entry.Method == http.MethodPost {
					body := w8aPostBodies[entry.Path]
					req = httptest.NewRequest(http.MethodPost, entry.Path, strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req = httptest.NewRequest(entry.Method, entry.Path, nil)
				}
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
				family := classifyFamily(entry.Path)
				lanes := snap.dispatches[family]
				if lanes[viaRelay] == nil {
					t.Errorf("%s: family %q dispatches = %v, want a relay lane row (never invented)", tc.name, family, lanes)
				}
			}
		})
	}
}

// TestW8aWaveJunkCookieStays401 pins the D2 follow-up lesson for the
// wave: a Cookie header without a session-cookie-shaped member is
// junk, not a browser session, and stays dest-401 with zero core
// contact.
func TestW8aWaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	targets := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v0/secrets-iam/posture-summary"},
		{http.MethodGet, "/api/v0/status/governance"},
		{http.MethodPost, "/api/v0/search/semantic"},
		{http.MethodPost, "/api/v0/terraform/config-state-drift/findings"},
	}
	for _, target := range targets {
		t.Run(target.method+" "+target.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			var req *http.Request
			if target.method == http.MethodPost {
				body := w8aPostBodies[target.path]
				req = httptest.NewRequest(http.MethodPost, target.path, strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(http.MethodGet, target.path, nil)
			}
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

// TestW8aWaveCredentiallessStays401 pins that a credential-less wave
// call gets the standing dest 401 with zero core contact.
func TestW8aWaveCredentiallessStays401(t *testing.T) {
	t.Parallel()

	for _, entry := range w8aWave {
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			var req *http.Request
			if entry.Method == http.MethodPost {
				body := w8aPostBodies[entry.Path]
				req = httptest.NewRequest(http.MethodPost, entry.Path, strings.NewReader(body))
			} else {
				req = httptest.NewRequest(http.MethodGet, entry.Path, nil)
			}
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

// TestW8aAuthorizerCountersPinned pins exactly one family authorizer
// allow per terminated call: the shared-key call files one allow
// judgment on its own family, never on a neighbor family and never
// twice.
func TestW8aAuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	for _, entry := range w8aWave {
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			var req *http.Request
			if entry.Method == http.MethodPost {
				req = w8aAuthedPost(entry.Path, "application/json")
			} else {
				req = authedRequest(http.MethodGet, entry.Path)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(entry.Path)
			if got := snap.authorizer[family][decisionAllow]; got != 1 {
				t.Errorf("family %q authorizer allow = %d, want exactly 1 per terminated call (got %v)", family, got, snap.authorizer)
			}
			for other, decisions := range snap.authorizer {
				if other == family {
					continue
				}
				if decisions[decisionAllow] != 0 {
					t.Errorf("neighbor family %q authorizer allow = %d, want 0 (one allow on the called family only)", other, decisions[decisionAllow])
				}
			}
		})
	}
}

// TestW8aDenyBranchShapePinned pins the unreachable-until-D8 deny
// branch: with allScopes false the terminated reader files the
// denied outcome on the native lane and answers the documented
// family 403 with zero core contact.
func TestW8aDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/secrets-iam/posture-summary", nil)
	rec := httptest.NewRecorder()
	srv.serveW8aTerminatedAs(rec, req, false)
	if rec.Code != http.StatusForbidden {
		t.Errorf("deny status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(body), w8aFamilyDenyMessage) {
		t.Errorf("deny body = %q, want the documented family-403 text %q", body, w8aFamilyDenyMessage)
	}
}
