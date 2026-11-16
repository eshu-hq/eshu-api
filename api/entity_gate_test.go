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

// Entity gate and authorizer proof (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#97). The wave gate answers every credential shape:
// shared-key terminates, foreign classes relay explicitly, junk and
// credential-less callers get the standing 401 with zero core contact,
// an empty configured key fails closed, and the unreachable-until-D8
// deny branch carries the documented family 403 without touching the
// core.

// TestEntityDenyBranchShapePinned pins the deny shape for the day dest
// gains a scoped/session resolver: a non-AllScopes identity is denied
// with the documented family 403 in both negotiated shapes and the
// core is never reached — for every proceed leg.
func TestEntityDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, leg := range entityParityLegs {
		if serveOf(leg.key) != entityServeGated {
			continue
		}
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.method + " " + leg.target + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				var req *http.Request
				if leg.method == http.MethodPost {
					req = authedRequest(http.MethodPost, leg.target)
					req.Body = io.NopCloser(strings.NewReader(leg.body))
					req.ContentLength = int64(len(leg.body))
				} else {
					req = authedRequest(http.MethodGet, leg.target)
				}
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				srv.serveEntityTerminatedAs(rec, req, false)

				res := rec.Result()
				if res.StatusCode != http.StatusForbidden {
					t.Fatalf("status = %d, want the documented 403", res.StatusCode)
				}
				body, _ := io.ReadAll(res.Body)
				if !strings.Contains(string(body), entityFamilyDenyMessage) {
					t.Errorf("body = %q, want it to carry the documented deny text", body)
				}
			})
		}
	}
}

// serveOf resolves a wave key to its serve kind.
func serveOf(key string) string {
	for _, entry := range entityWave {
		if entry.Method+" "+entry.Path == key {
			return entry.Serve
		}
	}
	return ""
}

// TestEntityWaveRelayExplicitForUnresolvableIdentities pins the gated
// forward-model never-invent rule for the proceed legs: each entity
// route carrying a credential class dest cannot resolve (a scoped
// token, an OIDC bearer, a browser session cookie) must relay
// explicitly to the core — the core judges, core bytes come back
// untouched — and must never answer a dest-invented 401, 403, 404, or
// 500.
func TestEntityWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, leg := range entityParityLegs {
		if serveOf(leg.key) != entityServeGated {
			continue
		}
		for _, tc := range []struct {
			name   string
			header map[string]string
		}{
			{"scoped bearer", map[string]string{"Authorization": "Bearer v2.scoped.token"}},
			{"oidc bearer", map[string]string{"Authorization": "Bearer oidc.jwt.token"}},
			{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
			{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
		} {
			name := leg.method + " " + leg.target + " " + tc.name
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				var req *http.Request
				if leg.method == http.MethodPost {
					req = httptest.NewRequest(http.MethodPost, leg.target, strings.NewReader(leg.body))
				} else {
					req = httptest.NewRequest(http.MethodGet, leg.target, nil)
				}
				for k, v := range tc.header {
					req.Header.Set(k, v)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Fatalf("%s: upstream hits = %d, want 1 (relay explicitly, never gate-invent)", tc.name, core.hits)
				}
				if rec.Code != http.StatusOK {
					t.Fatalf("%s: status = %d, want the relayed core 200 (never 401/403/404/500-invented)", tc.name, rec.Code)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != core.body {
					t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, body, core.body)
				}
			})
		}
	}
}

// TestEntityWaveJunkCookieStays401 pins the D2 follow-up cookie
// lesson: only session-cookie-shaped credentials relay. A Cookie
// header with no eshu_session (or __Host-eshu_session) member is not
// a browser session — it stays dest-401 with zero core contact, for
// every proceed leg in both negotiated shapes.
func TestEntityWaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	for _, leg := range entityParityLegs {
		if serveOf(leg.key) != entityServeGated {
			continue
		}
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.method + " " + leg.target + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, skeletonTestKey)
				handler := srv.Handler(DefaultV0Sunset)

				var req *http.Request
				if leg.method == http.MethodPost {
					req = httptest.NewRequest(http.MethodPost, leg.target, strings.NewReader(leg.body))
				} else {
					req = httptest.NewRequest(http.MethodGet, leg.target, nil)
				}
				req.Header.Set("Cookie", "junk=1; other=2")
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusUnauthorized {
					t.Errorf("status = %d, want the standing dest 401", rec.Code)
				}
				if core.hits != 0 {
					t.Errorf("upstream hits = %d, want 0 (junk cookies never reach core)", core.hits)
				}
			})
		}
	}
}

// TestEntityTerminatedAs_NilCore503 pins that the native routes with
// no core origin configured answer the permanent local 503 in both
// negotiated shapes, with zero upstream contact — for every proceed
// leg.
func TestEntityTerminatedAs_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)

	for _, leg := range entityParityLegs {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.method + " " + leg.target + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				var req *http.Request
				if leg.method == http.MethodPost {
					req = authedRequest(http.MethodPost, leg.target)
					req.Body = io.NopCloser(strings.NewReader(leg.body))
					req.ContentLength = int64(len(leg.body))
				} else {
					req = authedRequest(http.MethodGet, leg.target)
				}
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				if serveOf(leg.key) == entityServeGated {
					srv.serveEntityTerminatedAs(rec, req, true)
				} else {
					srv.Handler(DefaultV0Sunset).ServeHTTP(rec, req)
				}

				res := rec.Result()
				if res.StatusCode != http.StatusServiceUnavailable {
					t.Fatalf("status = %d, want the permanent local 503", res.StatusCode)
				}
				if res.Header.Get("Retry-After") != "" {
					t.Errorf("Retry-After = %q, want empty (local 503 never carries it)", res.Header.Get("Retry-After"))
				}
			})
		}
	}
}

// TestEntityGate_EmptyKeyFailsClosed pins that an empty configured key
// fails closed like RequireAuth: every proceed leg answers the
// standing 401 with zero core contact.
func TestEntityGate_EmptyKeyFailsClosed(t *testing.T) {
	t.Parallel()

	for _, leg := range entityParityLegs {
		if serveOf(leg.key) != entityServeGated {
			continue
		}
		name := leg.method + " " + leg.target
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, "")
			handler := srv.Handler(DefaultV0Sunset)

			var req *http.Request
			if leg.method == http.MethodPost {
				req = authedRequest(http.MethodPost, leg.target)
				req.Body = io.NopCloser(strings.NewReader(leg.body))
				req.ContentLength = int64(len(leg.body))
			} else {
				req = authedRequest(http.MethodGet, leg.target)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing 401", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (empty key never reaches core)", core.hits)
			}
		})
	}
}
