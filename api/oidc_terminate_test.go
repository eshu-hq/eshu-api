// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// OIDC-pair native termination proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72, oidc-pair lane, second of the
// sequential termination waves). GET /api/v0/auth/oidc/login and
// GET /api/v0/auth/oidc/callback terminate dest-side: the two exact
// pre-auth public paths register outside every gate ahead of the
// generic relay, each makes exactly one anonymous core fetch over
// D0-proven transport with the caller query crossing as opaque bytes,
// and dest synthesizes the answer from the port result (redirect
// status+Location byte-verbatim, Set-Cookie name/value/attrs/expiry
// verbatim, error envelopes verbatim). Login-state and session stores
// stay core-side: dest holds no store handle anywhere in this lane.
// TDD RED first: this test names handlers that do not exist yet, so it
// must fail to compile before the lane lands.
func TestOIDCTerminated_LoginSynthesizesRedirect(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{
			status: http.StatusFound,
			header: http.Header{"Location": []string{"https://idp.example.com/authorize?state=opaque&nonce=opaque"}},
			body:   ``,
		}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/login?provider_config_id=okta-dev&tenant_id=tenant_a&workspace_id=workspace_a&return_to=%2Fconsole", nil)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Fatalf("Accept %q: status = %d, want terminated 302", accept, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != "https://idp.example.com/authorize?state=opaque&nonce=opaque" {
			t.Errorf("Accept %q: Location = %q, want core value byte-verbatim", accept, got)
		}
		if core.hits != 1 {
			t.Errorf("Accept %q: upstream hits = %d, want exactly 1 (no retry on login start)", accept, core.hits)
		}
		if core.lastQuery != "provider_config_id=okta-dev&tenant_id=tenant_a&workspace_id=workspace_a&return_to=%2Fconsole" {
			t.Errorf("Accept %q: upstream query = %q, want opaque caller query", accept, core.lastQuery)
		}
	}
}

// TestOIDCTerminated_CallbackSynthesizesSession pins the terminated
// callback: the core 201 (session JSON plus both session cookies)
// synthesizes byte-verbatim with exactly one upstream attempt.
func TestOIDCTerminated_CallbackSynthesizesSession(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{
			status: http.StatusCreated,
			header: http.Header{"Set-Cookie": []string{
				"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict",
				"__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict",
			}},
			body: `{"csrf_token":"opaque-csrf"}`,
		}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/callback?state=st-opaque-123&code=cd-opaque-456", nil)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("Accept %q: status = %d, want terminated 201", accept, rec.Code)
		}
		cookies := rec.Header().Values("Set-Cookie")
		if len(cookies) != 2 {
			t.Fatalf("Accept %q: Set-Cookie count = %d, want 2 verbatim", accept, len(cookies))
		}
		raw, _ := io.ReadAll(rec.Result().Body)
		if string(raw) != `{"csrf_token":"opaque-csrf"}` {
			t.Errorf("Accept %q: body = %q, want core bytes verbatim", accept, raw)
		}
		if core.hits != 1 {
			t.Errorf("Accept %q: upstream hits = %d, want exactly 1 (no retry of callback mutation)", accept, core.hits)
		}
	}
}
