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

// Signinpolicy-group public-registration exactness (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#72). Only the two
// exact GET paths leave RequireAuth. This file proves the negative
// space: the admin GET and the PATCH stay authed, and every twin,
// subtree, and non-GET spelling of the two public paths keeps the
// standing gate behavior with zero widening. It also carries the
// missing-gate RED proof the D10 follow-up required: every keyless
// denial below asserts zero core contact.

// TestSigninpolicyGroup_AdminStaysAuthed pins no widening on the
// admin leg: keyless admin GET and keyless PATCH each keep the
// standing 401 with zero core contact, in both shapes. The keyed
// admin GET still forwards on its wave-gated lane (covered by
// TestSignInPolicyNative_ReadsStillProxy); the keyed PATCH still
// write-throughs (covered by the signin dispatch suite).
func TestSigninpolicyGroup_AdminStaysAuthed(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{status: http.StatusOK, body: `{"require_sso":false}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		for _, target := range []string{
			"GET /api/v0/auth/admin/sign-in-policy",
			"PATCH /api/v0/auth/admin/sign-in-policy",
			"GET /api/v0/auth/admin/sign-in-policy/",
			"GET /api/v1/auth/admin/sign-in-policy",
		} {
			method, path, _ := strings.Cut(target, " ")
			req := httptest.NewRequest(method, path, nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s Accept %q: status = %d, want the standing 401 (admin stays authed)", target, accept, rec.Code)
			}
		}
		if core.hits != 0 {
			t.Errorf("Accept %q: upstream hits = %d, want 0 (gate denials never dispatch)", accept, core.hits)
		}
	}
}

// TestSigninpolicyGroup_PublicExactnessRED pins the gate RED proof
// for the two public paths: every spelling outside the two exact
// GETs keeps the standing 401 for keyless callers with zero core
// contact, in both shapes — twins, subtrees, methods, and the v1
// twins (rewritten twins fall to the relay behind RequireAuth).
func TestSigninpolicyGroup_PublicExactnessRED(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{status: http.StatusOK, body: `{"require_sso":false}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		for _, target := range []string{
			"GET /api/v0/auth/sign-in-policy/",
			"GET /api/v0/auth/sign-in-policy/extra",
			"POST /api/v0/auth/sign-in-policy",
			"PUT /api/v0/auth/sign-in-policy",
			"DELETE /api/v0/auth/sign-in-policy",
			"PATCH /api/v0/auth/sign-in-policy",
			"GET /api/v0/auth/providers/",
			"GET /api/v0/auth/providers/extra",
			"POST /api/v0/auth/providers",
			"PUT /api/v0/auth/providers",
			"DELETE /api/v0/auth/providers",
			"GET /api/v1/auth/sign-in-policy/",
			"GET /api/v1/auth/providers/extra",
		} {
			method, path, _ := strings.Cut(target, " ")
			req := httptest.NewRequest(method, path, nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s Accept %q: status = %d, want the standing 401 (exact-only carve-out)", target, accept, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("%s Accept %q: WWW-Authenticate = %q, want the bare Bearer challenge", target, accept, got)
			}
		}
		if core.hits != 0 {
			t.Errorf("Accept %q: upstream hits = %d, want 0 (gate denials never dispatch)", accept, core.hits)
		}
	}
}

// TestSigninpolicyGroup_KeyedExactIsNative pins that keyed callers on
// the exact paths also terminate (synthesis ignores credentials —
// there is no gate to judge — and no credential crosses to the
// core). The keyed twins stay relay: see
// TestProviders_KeyedTwinRelaysNotSynthesizes for the providers twin
// and the sign-in-policy twin below.
func TestSigninpolicyGroup_KeyedExactIsNative(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/v0/auth/providers" {
			_, _ = io.WriteString(w, `{"providers":[],"local_login_offered":true,"self_service_tokens_offered":true}`)
			return
		}
		_, _ = io.WriteString(w, `{"require_sso":true}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range []struct {
		path string
		want string
	}{
		{"/api/v0/auth/sign-in-policy?tenant_id=tenant_a", `{"require_sso":true}`},
		{"/api/v0/auth/providers?tenant_id=tenant_a", `{"providers":[],"local_login_offered":false,"self_service_tokens_offered":true}`},
	} {
		req := authedRequest(http.MethodGet, tc.path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("keyed %s: status = %d, want terminated 200", tc.path, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if string(body) != tc.want {
			t.Errorf("keyed %s: body = %q, want the synthesized %q", tc.path, body, tc.want)
		}
	}
}

// TestSigninpolicyGroup_SignInPolicyKeyedTwinRelays pins the
// sign-in-policy twin boundary from the relay side: a keyed
// trailing-slash twin does NOT hit the synthesis — it crosses the
// generic relay with the core bytes and headers intact.
func TestSigninpolicyGroup_SignInPolicyKeyedTwinRelays(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusOK,
		body:   `{"require_sso":true}`,
		header: http.Header{"Cache-Control": []string{"private"}},
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v0/auth/sign-in-policy/")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("keyed twin status = %d, want the relayed 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private" {
		t.Errorf("keyed twin Cache-Control = %q, want the relayed core value (not synthesis)", got)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != core.body {
		t.Errorf("keyed twin body = %q, want the core bytes %q", body, core.body)
	}
	if core.lastAuth == "" {
		t.Error("keyed twin upstream Authorization empty, want the forwarded credential on the relay")
	}
}
