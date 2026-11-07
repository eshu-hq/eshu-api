// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestD9Public_CarveOutServesKeyless pins the C-c1 carve-out: the 2
// OIDC GETs serve credential-less callers — no dest 401 — and forward
// once to the same core route with the opaque caller query. This is
// the behavior change the carve-out exists for: before D9 these paths
// fell through to the generic relay inside RequireAuth and answered
// dest 401s without ever reaching core.
func TestD9Public_CarveOutServesKeyless(t *testing.T) {
	t.Parallel()

	for _, tc := range d9PublicCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://idp.example.com/authorize"}}, body: ``}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(tc.method, tc.path+"?"+tc.query, nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Errorf("%s Accept %q: upstream hits = %d, want exactly 1 (keyless carve-out forwards)", tc.name, accept, core.hits)
			}
			if core.lastMethod != tc.method || core.lastPath != tc.path {
				t.Errorf("%s Accept %q: upstream = %s %s, want the same core route", tc.name, accept, core.lastMethod, core.lastPath)
			}
			if core.lastQuery != tc.query {
				t.Errorf("%s Accept %q: upstream query = %q, want the opaque caller query", tc.name, accept, core.lastQuery)
			}
			if core.lastAuth != "" {
				t.Errorf("%s Accept %q: upstream Authorization = %q, want empty (no shared-key injection on public routes)",
					tc.name, accept, core.lastAuth)
			}
			if rec.Code != http.StatusFound {
				t.Errorf("%s Accept %q: status = %d, want relayed 302", tc.name, accept, rec.Code)
			}
		}
	}
}

// TestD9Public_CarveOutIsExactlyTwo pins the carve-out boundary: only
// the two exact GET paths leave the gate. Near-miss paths fall through
// to the keyed generic relay and keep the standing 401 for
// credential-less callers with zero core contact; non-GET spellings of
// the carved paths are not carved out either (core's bypass is
// GET-only); and the GitHub/SAML login paths stay gated for their own
// sequential lanes.
func TestD9Public_CarveOutIsExactlyTwo(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"GET /api/v0/auth/oidc/logins",
		"GET /api/v0/auth/oidc/login/extra",
		"POST /api/v0/auth/oidc/login",
		"POST /api/v0/auth/oidc/callback",
		"GET /api/v0/auth/oidc/callback/extra",
		"GET /api/v0/auth/github/login",
		"GET /api/v0/auth/github/callback",
		"GET /api/v0/auth/saml/providers/p1/login",
	} {
		method, path, _ := strings.Cut(target, " ")
		req := httptest.NewRequest(method, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want the standing 401 (not carved out)", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (gate denials never dispatch)", core.hits)
	}
}

// TestD9Public_AuthedCallerKeyCrossesUntouched pins that a shared-key
// caller on a public route forwards its own key as an ordinary header:
// dest neither strips it nor judges it there — core judges every
// credential it receives.
func TestD9Public_AuthedCallerKeyCrossesUntouched(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://idp.example.com/authorize"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v0/auth/oidc/login?provider_config_id=okta-dev")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want 1", core.hits)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization = %q, want the caller key untouched", core.lastAuth)
	}
	if core.lastQuery != "provider_config_id=okta-dev" {
		t.Errorf("upstream query = %q, want the opaque caller query", core.lastQuery)
	}
}

// TestD9Public_SlashSpellingStaysRelayed pins the exact-path carve-out
// boundary: the slash spellings of the OIDC routes are NOT carved out
// (a slash twin would be a subtree swallowing past the bypass).
// Keyless callers keep the standing 401 with zero core contact; keyed
// callers ride the generic relay and get core's answer for the spelled
// path.
func TestD9Public_SlashSpellingStaysRelayed(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{
		"/api/v0/auth/oidc/login/",
		"/api/v0/auth/oidc/callback/",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s keyless: status = %d, want the standing 401", path, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (slash spellings never bypass)", core.hits)
	}

	for _, path := range []string{
		"/api/v0/auth/oidc/login/",
		"/api/v0/auth/oidc/callback/",
	} {
		req := authedRequest(http.MethodGet, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if core.hits == 0 {
			t.Fatalf("GET %s keyed: no upstream hit, want the generic relay to forward", path)
		}
		if core.lastPath != path {
			t.Errorf("GET %s keyed: upstream path = %q, want the spelled path relayed", path, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s keyed: status = %d, want relayed 200", path, rec.Code)
		}
	}
}

// TestD9Native_V1AliasOrderingUnchanged pins that the v1 alias rewrite
// lands the OIDC GETs on their native handlers: the keyless v1 login
// reaches the public carve-out with the rewritten /api/v0/* path.
func TestD9Native_V1AliasOrderingUnchanged(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://idp.example.com/authorize"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	public := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login?provider_config_id=okta-dev", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, public)

	if core.hits != 1 || core.lastPath != "/api/v0/auth/oidc/login" {
		t.Errorf("public: hits = %d path = %q, want alias rewrite to the carve-out v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("public: Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusFound {
		t.Errorf("public: status = %d, want relayed 302", rec.Code)
	}
}

// TestD9Native_NeighborLanesStillRelay pins that the GitHub and SAML
// login routes are unaffected by the OIDC registrations: a keyless
// GitHub login keeps the standing 401 with zero core contact (its lane
// has not landed), and a keyed one rides the generic relay with
// exactly one upstream hit. The OIDC paths share no registration key
// with those lanes beyond /api/v0/auth, and Go matches method and
// path together, so the lanes never merge.
func TestD9Native_NeighborLanesStillRelay(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	keyless := httptest.NewRequest(http.MethodGet, "/api/v0/auth/github/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, keyless)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("keyless github login: status = %d, want the standing 401", rec.Code)
	}
	if core.hits != 0 {
		t.Errorf("keyless github login: upstream hits = %d, want 0", core.hits)
	}

	keyed := authedRequest(http.MethodGet, "/api/v0/auth/github/login")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, keyed)
	if core.hits != 1 || core.lastPath != "/api/v0/auth/github/login" {
		t.Errorf("keyed github login: hits = %d path = %q, want the generic relay to forward once untouched",
			core.hits, core.lastPath)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("keyed github login: status = %d, want 200", rec.Code)
	}
}

// TestD9Public_QueryCarriesNoBody pins that the login GETs forward no
// caller body: the credential material rides the query string only,
// and dest invents no body of its own.
func TestD9Public_QueryCarriesNoBody(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://idp.example.com/authorize"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/callback?state=st-opaque-123&code=cd-opaque-456", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want 1", core.hits)
	}
	if core.lastBody != "" {
		t.Errorf("upstream body = %q, want empty (login GETs carry no body)", core.lastBody)
	}
}
