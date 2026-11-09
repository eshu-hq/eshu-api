// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// GitHub-pair termination registration proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72). The carve-out is exactly the two
// GETs: every near-miss spelling stays behind RequireAuth on the
// generic relay, OIDC stays on its own landed termination lane and
// SAML stays on its landed write-through lane, and the v1 alias
// rewrites onto the terminated handlers.

// TestGitHubTermGroup_PublicExactnessRED pins the carve-out boundary:
// only the two exact GET paths terminate. Near-miss paths fall
// through to the keyed generic relay and keep the standing 401 for
// credential-less callers with zero core contact; non-GET spellings
// of the carved paths are not carved out either (core mounts
// GET-only).
func TestGitHubTermGroup_PublicExactnessRED(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"GET /api/v0/auth/github/login/",
		"GET /api/v0/auth/github/login/extra",
		"GET /api/v0/auth/github/callback/",
		"GET /api/v0/auth/github/callback/extra",
		"POST /api/v0/auth/github/login",
		"POST /api/v0/auth/github/callback",
		"PUT /api/v0/auth/github/login",
		"DELETE /api/v0/auth/github/callback",
		"GET /api/v0/auth/github/logins",
		"GET /api/v0/auth/github/callbacks",
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

// TestGitHubTermGroup_KeyedExactIsNative pins that keyed callers on
// the exact paths ride the terminated lane: the dispatch signal files
// via="native" (not the relay fallback) with the core bytes
// synthesized from the port.
func TestGitHubTermGroup_KeyedExactIsNative(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusFound,
		header: http.Header{"Location": []string{"https://github.com/login/oauth/authorize?state=opaque"}},
		body:   ``,
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{githubLoginPath, githubCallbackPath} {
		req := authedRequest(http.MethodGet, path+"?"+d9GitHubParityQuery(path))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("GET %s: status = %d, want terminated 302", path, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != "https://github.com/login/oauth/authorize?state=opaque" {
			t.Errorf("GET %s: Location = %q, want core value byte-verbatim", path, got)
		}
	}
	if core.hits != 2 {
		t.Errorf("upstream hits = %d, want 2 (one port fetch per terminated leg)", core.hits)
	}
}

// TestGitHubTermGroup_KeyedTwinRelays pins that the trailing-slash
// twins of the terminated paths stay relayed for keyed callers: the
// twin answers core's bytes for the spelled path (never synthesized)
// with the relay dispatch signal, exactly like before the lane.
func TestGitHubTermGroup_KeyedTwinRelays(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusTeapot, body: `{"spelled":"twin"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{githubLoginPath + "/", githubCallbackPath + "/"} {
		req := authedRequest(http.MethodGet, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusTeapot {
			t.Errorf("GET %s: status = %d, want relayed core status (twin stays relay)", path, rec.Code)
		}
		if core.lastPath != path {
			t.Errorf("GET %s: upstream path = %q, want the spelled twin path relayed", path, core.lastPath)
		}
	}
}

// TestGitHubTermGroup_OIDCSAMLUntouched pins the family boundary:
// the OIDC pair still rides its landed termination lane and the SAML
// triple still rides its landed write-through lane (keyless, one
// forward each). This lane moves only the GitHub pair.
func TestGitHubTermGroup_OIDCSAMLUntouched(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://github.com/login/oauth/authorize"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	targets := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v0/auth/oidc/login?provider_config_id=okta-dev", ``},
		{http.MethodGet, "/api/v0/auth/oidc/callback?state=opaque&code=opaque", ``},
		{http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/metadata", ``},
		{http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", ``},
	}
	for _, tc := range targets {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.body != "" {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("%s %s: status = %d, want relayed 302 (lane untouched)", tc.method, tc.path, rec.Code)
		}
	}
	acs := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader("RelayState=r&SAMLResponse=s"))
	acs.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	acsRec := httptest.NewRecorder()
	handler.ServeHTTP(acsRec, acs)
	if acsRec.Code != http.StatusFound {
		t.Errorf("POST ACS: status = %d, want relayed 302 (lane untouched)", acsRec.Code)
	}
	if core.hits != len(targets)+1 {
		t.Errorf("upstream hits = %d, want %d (one fetch per untouched route)", core.hits, len(targets)+1)
	}
}

// TestGitHubTermGroup_V1AliasTerminates pins that the v1 alias
// rewrites onto the terminated handlers: both v1 spellings terminate
// with the core bytes, exactly like their v0 twins.
func TestGitHubTermGroup_V1AliasTerminates(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusFound,
		header: http.Header{"Location": []string{"https://github.com/login/oauth/authorize?state=opaque"}},
		body:   ``,
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{
		"/api/v1/auth/github/login?provider_config_id=github-dev",
		"/api/v1/auth/github/callback?state=opaque&code=opaque",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("GET %s: status = %d, want terminated 302 via v1 alias", path, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != "https://github.com/login/oauth/authorize?state=opaque" {
			t.Errorf("GET %s: Location = %q, want core value byte-verbatim", path, got)
		}
	}
	if core.hits != 2 {
		t.Errorf("upstream hits = %d, want 2 (one port fetch per aliased leg)", core.hits)
	}
}
