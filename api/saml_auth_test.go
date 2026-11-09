// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// SAML-triple termination registration proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72). The carve-out is exactly the three
// SAML paths: every near-miss spelling stays behind RequireAuth on
// the generic relay, OIDC and GitHub stay on their landed termination
// lanes, and the v1 alias rewrites onto the terminated handlers.

// TestSAMLTermGroup_PublicExactnessRED pins the carve-out boundary:
// only the three exact SAML paths terminate. Near-miss paths fall
// through to the keyed generic relay and keep the standing 401 for
// credential-less callers with zero core contact; non-GET spellings
// of the metadata/login paths (and non-POST spellings of the ACS
// path) are not carved out either (core mounts GET-only on
// metadata/login and POST-only on ACS); the OIDC and GitHub pairs
// stay on their own landed lanes.
func TestSAMLTermGroup_PublicExactnessRED(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"GET /api/v0/auth/saml/providers",
		"GET /api/v0/auth/saml/providers/p1/logins",
		"GET /api/v0/auth/saml/providers/p1/login/extra",
		"GET /api/v0/auth/saml/providers/p1/metadata/extra",
		"POST /api/v0/auth/saml/providers/p1/acs/extra",
		"POST /api/v0/auth/saml/providers/p1/metadata",
		"POST /api/v0/auth/saml/providers/p1/login",
		"GET /api/v0/auth/saml/providers/p1/acs",
		"PUT /api/v0/auth/saml/providers/p1/acs",
		"DELETE /api/v0/auth/saml/providers/p1/metadata",
		"GET /api/v0/auth/saml/providers/p1/metadata/",
		"GET /api/v0/auth/saml/providers/p1/login/",
		"POST /api/v0/auth/saml/providers/p1/acs/",
	} {
		method, path, _ := strings.Cut(target, " ")
		var req *http.Request
		if method == http.MethodPost || method == http.MethodPut {
			req = httptest.NewRequest(method, path, strings.NewReader("RelayState=opaque& SAMLResponse=opaque"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
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

// TestSAMLTermGroup_KeyedExactIsNative pins that keyed callers on the
// exact paths ride the terminated lane: the dispatch signal files
// via="native" (not the relay fallback) with the core bytes
// synthesized from the port.
func TestSAMLTermGroup_KeyedExactIsNative(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusFound,
		header: http.Header{"Location": []string{"https://idp.example.com/sso?SAMLRequest=opaque"}},
		body:   ``,
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	targets := []struct {
		method string
		target string
		body   string
	}{
		{http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/metadata", ""},
		{http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", ""},
	}
	for _, tc := range targets {
		req := authedRequest(tc.method, tc.target)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("%s %s: status = %d, want terminated 302", tc.method, tc.target, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != "https://idp.example.com/sso?SAMLRequest=opaque" {
			t.Errorf("%s %s: Location = %q, want core value byte-verbatim", tc.method, tc.target, got)
		}
	}
	acsBody := strings.NewReader("RelayState=opaque& SAMLResponse=opaque")
	acsReq := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", acsBody)
	acsReq.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	acsReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	acsRec := httptest.NewRecorder()
	handler.ServeHTTP(acsRec, acsReq)
	if acsRec.Code != http.StatusFound {
		t.Errorf("POST ACS: status = %d, want terminated 302", acsRec.Code)
	}
	if core.hits != 3 {
		t.Errorf("upstream hits = %d, want 3 (one port fetch per terminated leg)", core.hits)
	}
}

// TestSAMLTermGroup_KeyedTwinRelays pins that the trailing-slash
// twins of the terminated paths stay relayed for keyed callers: the
// twin answers core's bytes for the spelled path (never synthesized)
// with the relay dispatch signal, exactly like before the lane.
func TestSAMLTermGroup_KeyedTwinRelays(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusTeapot, body: `{"spelled":"twin"}`}
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
		{http.MethodGet, "/api/v0/auth/saml/providers/p1/metadata/", ""},
		{http.MethodGet, "/api/v0/auth/saml/providers/p1/login/", ""},
		{http.MethodPost, "/api/v0/auth/saml/providers/p1/acs/", "RelayState=opaque& SAMLResponse=opaque"},
	}
	for _, tc := range targets {
		var req *http.Request
		if tc.body != "" {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusTeapot {
			t.Errorf("%s %s: status = %d, want relayed core status (twin stays relay)", tc.method, tc.path, rec.Code)
		}
		if core.lastPath != tc.path {
			t.Errorf("%s %s: upstream path = %q, want the spelled twin path relayed", tc.method, tc.path, core.lastPath)
		}
	}
}

// TestSAMLTermGroup_OIDCGitHubUntouched pins the family boundary:
// the OIDC and GitHub pairs stay on their landed termination lanes
// (keyless, one fetch each). This lane moves only the SAML triple.
func TestSAMLTermGroup_OIDCGitHubUntouched(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://idp.example.com/authorize"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	targets := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v0/auth/oidc/login?provider_config_id=okta-dev"},
		{http.MethodGet, "/api/v0/auth/oidc/callback?state=opaque&code=opaque"},
		{http.MethodGet, "/api/v0/auth/github/login?provider_config_id=gh-dev"},
		{http.MethodGet, "/api/v0/auth/github/callback?state=opaque&code=opaque"},
	}
	for _, tc := range targets {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("%s %s: status = %d, want terminated 302 (lane untouched)", tc.method, tc.path, rec.Code)
		}
	}
	if core.hits != len(targets) {
		t.Errorf("upstream hits = %d, want %d (one fetch per untouched route)", core.hits, len(targets))
	}
}

// TestSAMLTermGroup_V1AliasTerminates pins that the v1 alias
// rewrites onto the terminated handlers: the v1 spellings of all
// three legs terminate with the core bytes, exactly like their v0
// twins.
func TestSAMLTermGroup_V1AliasTerminates(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusFound,
		header: http.Header{"Location": []string{"https://idp.example.com/sso?SAMLRequest=opaque"}},
		body:   ``,
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v1/auth/saml/providers/acme-munich/metadata",
		"/api/v1/auth/saml/providers/acme-munich/login?return_to=%2Fconsole",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Errorf("GET %s: status = %d, want terminated 302 via v1 alias", target, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != "https://idp.example.com/sso?SAMLRequest=opaque" {
			t.Errorf("GET %s: Location = %q, want core value byte-verbatim", target, got)
		}
	}
	acs := httptest.NewRequest(http.MethodPost, "/api/v1/auth/saml/providers/acme-munich/acs", strings.NewReader("RelayState=opaque& SAMLResponse=opaque"))
	acs.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	acsRec := httptest.NewRecorder()
	handler.ServeHTTP(acsRec, acs)
	if acsRec.Code != http.StatusFound {
		t.Errorf("POST %s: status = %d, want terminated 302 via v1 alias", "/api/v1/auth/saml/providers/acme-munich/acs", acsRec.Code)
	}
	if core.hits != 3 {
		t.Errorf("upstream hits = %d, want 3 (one port fetch per aliased leg)", core.hits)
	}
	if core.lastPath != "/api/v0/auth/saml/providers/acme-munich/acs" {
		t.Errorf("ACS upstream path = %q, want alias rewrite to the v0 route", core.lastPath)
	}
}
