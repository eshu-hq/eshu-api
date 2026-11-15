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

// Repository story gate and authorizer proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#97). The wave gate answers every
// credential shape: shared-key terminates, foreign classes relay
// explicitly, junk and credential-less callers get the standing 401
// with zero core contact, an empty configured key fails closed, and
// the unreachable-until-D8 deny branch carries the documented family
// 403 without touching the core.

// TestStoryDenyBranchShapePinned pins the deny shape for the day dest
// gains a scoped/session resolver: a non-AllScopes identity is denied
// with the documented family 403 in both negotiated shapes and the
// core is never reached.
func TestStoryDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		t.Run(accept, func(t *testing.T) {
			t.Parallel()

			req := authedRequest(http.MethodGet, storyConcretePath)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			srv.serveStoryTerminatedAs(rec, req, false)

			res := rec.Result()
			if res.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want the documented 403", res.StatusCode)
			}
			body, _ := io.ReadAll(res.Body)
			if !strings.Contains(string(body), storyFamilyDenyMessage) {
				t.Errorf("body = %q, want it to carry the documented deny text", body)
			}
		})
	}
}

// TestStoryWaveRelayExplicitForUnresolvableIdentities pins the gated
// forward-model never-invent rule: the story GET carrying a credential
// class dest cannot resolve (a scoped token, an OIDC bearer, a browser
// session cookie) must relay explicitly to the core — the core judges,
// core bytes come back untouched — and must never answer a
// dest-invented 401, 403, 404, or 500.
func TestStoryWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		header map[string]string
	}{
		{"scoped bearer", map[string]string{"Authorization": "Bearer v2.scoped.token"}},
		{"oidc bearer", map[string]string{"Authorization": "Bearer oidc.jwt.token"}},
		{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
		{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, storyConcretePath, nil)
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

// TestStoryWaveJunkCookieStays401 pins the D2 follow-up cookie lesson:
// only session-cookie-shaped credentials relay. A Cookie header with
// no eshu_session (or __Host-eshu_session) member stays dest-401 with
// zero core contact, in both negotiated shapes.
func TestStoryWaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		t.Run(accept, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, storyConcretePath, nil)
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

// TestStoryTerminatedAs_NilCore503 pins that the story GET with no
// core origin configured answers the permanent local 503 in both
// negotiated shapes, with zero upstream contact.
func TestStoryTerminatedAs_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		t.Run(accept, func(t *testing.T) {
			t.Parallel()

			req := authedRequest(http.MethodGet, storyConcretePath)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			srv.serveStoryTerminatedAs(rec, req, true)

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

// TestStoryGate_EmptyKeyFailsClosed pins that an empty configured key
// fails closed like RequireAuth: the story GET answers the standing
// 401 with zero core contact.
func TestStoryGate_EmptyKeyFailsClosed(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, "")
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, storyConcretePath)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want the standing 401", rec.Code)
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (empty key never reaches core)", core.hits)
	}
}
