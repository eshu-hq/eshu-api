// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestD8Native_BootstrapKeepsSharedOperatorGate is the per-route review
// for POST /api/v0/auth/local/bootstrap, pinned as a test: a keyless
// caller gets the standing dest 401 with zero core contact (the gate
// never admits what core must judge); a wrong-key caller relays
// explicitly so core judges the credential (dest invents no failure
// for it); a keyed call forwards exactly once, and core's operator
// judgment (the 403 the shared-operator gate answers when the seal is
// closed) relays untouched. Dest invents no 410 and caches no seal.
func TestD8Native_BootstrapKeepsSharedOperatorGate(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusForbidden, body: `{"error":"Forbidden","detail":"setup is already complete"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	keyless := httptest.NewRequest(http.MethodPost, "/api/v0/auth/local/bootstrap", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, keyless)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("keyless bootstrap: status = %d, want the standing 401", rec.Code)
	}
	if core.hits != 0 {
		t.Errorf("keyless bootstrap: upstream hits = %d, want 0 (gate denials never dispatch)", core.hits)
	}

	wrongKey := httptest.NewRequest(http.MethodPost, "/api/v0/auth/local/bootstrap", nil)
	wrongKey.Header.Set("Authorization", "Bearer wrong-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, wrongKey)
	if core.hits != 1 {
		t.Fatalf("wrong-key bootstrap: upstream hits = %d, want 1 (explicit relay so core judges)", core.hits)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("wrong-key bootstrap: status = %d, want core's answer relayed", rec.Code)
	}

	keyed := authedRequest(http.MethodPost, "/api/v0/auth/local/bootstrap")
	keyed.Body = io.NopCloser(strings.NewReader(`{"username":"owner","password":"first-proof"}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, keyed)
	if core.hits != 2 {
		t.Fatalf("keyed bootstrap: upstream hits = %d, want 2 total (one forward per admitted caller)", core.hits)
	}
	if core.lastBody != `{"username":"owner","password":"first-proof"}` {
		t.Errorf("keyed bootstrap: upstream body = %q, want the opaque caller bytes", core.lastBody)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("keyed bootstrap: status = %d, want relayed 403 (core judges the operator seal)", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(body), "setup is already complete") {
		t.Errorf("keyed bootstrap: body = %q, want core's seal answer relayed", body)
	}
}

// TestD8Public_SlashSpellingStaysRelayed pins the exact-path carve-out
// boundary: the slash spellings of the public routes are NOT carved
// out (a slash twin would be a subtree swallowing /login/* past the
// bypass). Keyless callers keep the standing 401 with zero core
// contact; keyed callers ride the generic relay and get core's answer
// for the spelled path.
func TestD8Public_SlashSpellingStaysRelayed(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{
		"/api/v0/auth/local/login/",
		"/api/v0/auth/local/invitations/accept/",
		"/api/v0/auth/local/break-glass/session/",
		"/api/v0/auth/local/password/rotate/",
	} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("POST %s keyless: status = %d, want the standing 401", path, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (slash spellings never bypass)", core.hits)
	}

	for _, path := range []string{
		"/api/v0/auth/local/login/",
		"/api/v0/auth/local/invitations/accept/",
	} {
		req := authedRequest(http.MethodPost, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if core.hits == 0 {
			t.Fatalf("POST %s keyed: no upstream hit, want the generic relay to forward", path)
		}
		if core.lastPath != path {
			t.Errorf("POST %s keyed: upstream path = %q, want the spelled path relayed", path, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("POST %s keyed: status = %d, want relayed 200", path, rec.Code)
		}
	}
}

// TestD8Gate_ForeignCredentialRelaysExplicit pins the gate's middle
// branch: a scoped/OIDC-shaped Bearer (anything but the shared key)
// and a session-cookie-shaped request relay explicitly to the same
// core route on the relay lane, so core judges the class dest cannot
// resolve. Junk cookies and credential-less callers keep the standing
// 401 with zero core contact.
func TestD8Gate_ForeignCredentialRelaysExplicit(t *testing.T) {
	t.Parallel()

	newLanes := func(t *testing.T) (*Server, *stubCore, http.Handler) {
		t.Helper()
		core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		t.Cleanup(upstream.Close)
		tel := NewTelemetry(discardLogger())
		srv := NewServer(upstream.URL, skeletonTestKey)
		srv.telemetry = tel
		return srv, core, srv.Handler(DefaultV0Sunset)
	}

	t.Run("foreign bearer", func(t *testing.T) {
		t.Parallel()
		_, core, handler := newLanes(t)
		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/sessions", nil)
		req.Header.Set("Authorization", "Bearer scoped-token-class")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if core.hits != 1 || core.lastAuth != "Bearer scoped-token-class" {
			t.Errorf("hits = %d auth = %q, want the foreign Bearer relayed once untouched", core.hits, core.lastAuth)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want relayed 200", rec.Code)
		}
	})

	t.Run("session cookie", func(t *testing.T) {
		t.Parallel()
		_, core, handler := newLanes(t)
		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/browser-session", nil)
		req.AddCookie(&http.Cookie{Name: "eshu_session", Value: "opaque"})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if core.hits != 1 {
			t.Errorf("hits = %d, want the cookie caller relayed once so core judges it", core.hits)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want relayed 200", rec.Code)
		}
	})

	t.Run("junk cookie is 401", func(t *testing.T) {
		t.Parallel()
		_, core, handler := newLanes(t)
		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/browser-session", nil)
		req.AddCookie(&http.Cookie{Name: "other", Value: "junk"})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want the standing 401", rec.Code)
		}
		if core.hits != 0 {
			t.Errorf("hits = %d, want 0 (junk cookies never dispatch)", core.hits)
		}
	})

	t.Run("credentialless is 401", func(t *testing.T) {
		t.Parallel()
		_, core, handler := newLanes(t)
		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want the standing 401", rec.Code)
		}
		if core.hits != 0 {
			t.Errorf("hits = %d, want 0 (denials never dispatch)", core.hits)
		}
	})

	t.Run("empty key fails closed", func(t *testing.T) {
		t.Parallel()
		srv := NewServer("http://core.invalid", "")
		req := authedRequest(http.MethodGet, "/api/v0/auth/sessions")
		rec := httptest.NewRecorder()
		srv.Handler(DefaultV0Sunset).ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want the standing 401", rec.Code)
		}
	})

	t.Run("explicit relay nil core is 503", func(t *testing.T) {
		t.Parallel()
		srv := NewServer("", skeletonTestKey)
		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/sessions", nil)
		req.AddCookie(&http.Cookie{Name: "eshu_session", Value: "opaque"})
		rec := httptest.NewRecorder()
		srv.d8RelayExplicit(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want the permanent 503", rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Errorf("Retry-After = %q, want absent (permanent, never retry)", got)
		}
	})
}

// d8DuplicateStub mimics the core's current-state behavior behind the
// D8 routes: every delivery executes (dest dedups nothing), and the
// stub answers each delivery the same way so the test proves dest
// adds nothing — no dedup cache, no retry, no coalescing.
type d8DuplicateStub struct {
	t    *testing.T
	hits int
}

func (s *d8DuplicateStub) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			s.t.Errorf("stub read body: %v", err)
		}
		s.hits++
		_, _ = io.WriteString(w, `{"status":"accepted","echo":`+strconv.Quote(string(raw))+`}`)
	})
}

// TestD8Native_DuplicateIsNoWorseThanCore pins the tier-C proof-box
// duplicate rule: a retried clone lands exactly as it would against
// the core directly, because dest adds nothing — no dedup cache, no
// retry, no coalescing. Two identical deliveries forward twice (2
// upstream hits) and both land. The ledger deficit stays open in the
// contract: this test proves dest is no worse than direct core, not
// that a retried session-create is safe.
func TestD8Native_DuplicateIsNoWorseThanCore(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		route   string
		payload string
	}{
		{"POST /api/v0/auth/browser-session", `{"ttl":"8h"}`},
		{"DELETE /api/v0/auth/browser-session", ``},
		{"POST /api/v0/auth/local/login", `{"username":"operator","password":"s3cret-proof"}`},
		{"POST /api/v0/auth/local/api-tokens", `{"token_class":"personal"}`},
		{"POST /api/v0/auth/local/mfa/totp/confirm", `{"code":"123456"}`},
	} {
		method, path, _ := strings.Cut(tc.route, " ")
		stub := &d8DuplicateStub{t: t}
		upstream := httptest.NewServer(stub.handler())
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		var codes []int
		for i := 0; i < 2; i++ {
			var req *http.Request
			if tc.payload == "" {
				req = authedRequest(method, path)
			} else {
				req = authedRequest(method, path)
				req.Body = io.NopCloser(strings.NewReader(tc.payload))
			}
			// The public login route serves keyless callers; prove the
			// no-worse rule on the keyless shape too.
			if path == "/api/v0/auth/local/login" {
				req = httptest.NewRequest(method, path, strings.NewReader(tc.payload))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			codes = append(codes, rec.Code)
		}
		if stub.hits != 2 {
			t.Errorf("%s: upstream hits = %d, want 2 (dest forwards both, dedups neither)", tc.route, stub.hits)
		}
		if codes[0] != codes[1] {
			t.Errorf("%s: statuses = %v, want both deliveries to land the same", tc.route, codes)
		}
	}
}
