// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestD9GitHubPublic_CarveOutServesKeyless pins the C-c1 carve-out: the
// 2 GitHub GETs serve credential-less callers — no dest 401 — and
// forward once to the same core route with the opaque caller query.
// This is the behavior change the carve-out exists for: before this
// lane these paths fell through to the generic relay inside RequireAuth
// and answered dest 401s without ever reaching core.
func TestD9GitHubPublic_CarveOutServesKeyless(t *testing.T) {
	t.Parallel()

	for _, tc := range d9GitHubPublicCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://github.com/login/oauth/authorize"}}, body: ``}
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

// TestD9GitHubPublic_CarveOutIsExactlyTwo pins the carve-out boundary:
// only the two exact GitHub GET paths leave the gate. Near-miss paths
// fall through to the keyed generic relay and keep the standing 401 for
// credential-less callers with zero core contact; non-GET spellings of
// the carved paths are not carved out either (core's bypass is
// GET-only); the OIDC login paths stay on their own landed lane and
// SAML rides its own landed lane (d9SAMLRoutes).
func TestD9GitHubPublic_CarveOutIsExactlyTwo(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"GET /api/v0/auth/github/logins",
		"GET /api/v0/auth/github/login/extra",
		"POST /api/v0/auth/github/login",
		"POST /api/v0/auth/github/callback",
		"GET /api/v0/auth/github/callback/extra",
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

// TestD9GitHubPublic_AuthedCallerKeyCrossesUntouched pins that a
// shared-key caller on a public route forwards its own key as an
// ordinary header: dest neither strips it nor judges it there — core
// judges every credential it receives.
func TestD9GitHubPublic_AuthedCallerKeyCrossesUntouched(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://github.com/login/oauth/authorize"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v0/auth/github/login?provider_config_id=github-dev")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want 1", core.hits)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization = %q, want the caller key untouched", core.lastAuth)
	}
	if core.lastQuery != "provider_config_id=github-dev" {
		t.Errorf("upstream query = %q, want the opaque caller query", core.lastQuery)
	}
}

// TestD9GitHubPublic_SlashSpellingStaysRelayed pins the exact-path
// carve-out boundary: the slash spellings of the GitHub routes are NOT
// carved out (a slash twin would be a subtree swallowing past the
// bypass). Keyless callers keep the standing 401 with zero core
// contact; keyed callers ride the generic relay and get core's answer
// for the spelled path.
func TestD9GitHubPublic_SlashSpellingStaysRelayed(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{
		"/api/v0/auth/github/login/",
		"/api/v0/auth/github/callback/",
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
		"/api/v0/auth/github/login/",
		"/api/v0/auth/github/callback/",
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

// TestD9GitHubNative_V1AliasOrderingUnchanged pins that the v1 alias
// rewrite lands the GitHub GETs on their native handlers: the keyless
// v1 login reaches the public carve-out with the rewritten /api/v0/*
// path.
func TestD9GitHubNative_V1AliasOrderingUnchanged(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://github.com/login/oauth/authorize"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	public := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/login?provider_config_id=github-dev", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, public)

	if core.hits != 1 || core.lastPath != "/api/v0/auth/github/login" {
		t.Errorf("public: hits = %d path = %q, want alias rewrite to the carve-out v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("public: Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusFound {
		t.Errorf("public: status = %d, want relayed 302", rec.Code)
	}
}

// TestD9GitHubNative_NeighborLanes pins the lane boundaries around the
// GitHub registrations: the OIDC login routes stay on their own landed
// native lane (keyless OIDC login forwards once with the opaque query),
// and the SAML login routes stay on their own landed lane (keyless
// SAML login forwards once natively now — the inverted replacement for
// the relay-only SAML assertions this suite carried before the SAML
// lane landed; the full inverted coverage lives in the D9 SAML lane
// suite, TestD9SAMLNative_NeighborLanes). The GitHub paths share no
// registration key with those lanes beyond /api/v0/auth, and Go matches
// method and path together, so the lanes never merge.
func TestD9GitHubNative_NeighborLanes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	oidc := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/login?provider_config_id=okta-dev", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, oidc)
	if core.hits != 1 || core.lastPath != "/api/v0/auth/oidc/login" {
		t.Errorf("keyless oidc login: hits = %d path = %q, want the landed OIDC lane to forward once untouched",
			core.hits, core.lastPath)
	}

	keyless := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/p1/login", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, keyless)
	if core.hits != 2 || core.lastPath != "/api/v0/auth/saml/providers/p1/login" {
		t.Errorf("keyless saml login: hits = %d path = %q, want the landed SAML lane to forward once untouched",
			core.hits, core.lastPath)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("keyless saml login: status = %d, want 200", rec.Code)
	}

	keyed := authedRequest(http.MethodGet, "/api/v0/auth/saml/providers/p1/login")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, keyed)
	if core.hits != 3 || core.lastPath != "/api/v0/auth/saml/providers/p1/login" {
		t.Errorf("keyed saml login: hits = %d path = %q, want the landed SAML lane to forward once untouched",
			core.hits, core.lastPath)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("keyed saml login: status = %d, want 200", rec.Code)
	}
}

// TestD9GitHubPublic_QueryCarriesNoBody pins that the login GETs forward
// no caller body: the credential material rides the query string only,
// and dest invents no body of its own.
func TestD9GitHubPublic_QueryCarriesNoBody(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://github.com/login/oauth/authorize"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/github/callback?state=st-opaque-123&code=cd-opaque-456", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want 1", core.hits)
	}
	if core.lastBody != "" {
		t.Errorf("upstream body = %q, want empty (login GETs carry no body)", core.lastBody)
	}
}
