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

// SAML-triple resilience conformance (companion to
// saml_conformance_test.go): single-attempt, dead-core,
// unconfigured-origin, disabled-provider, unknown-provider,
// sibling-relay, and no-authorizer proofs.

// TestSAMLTerm_NoRetryOnUnstable pins the single-attempt rule: the
// core 500, the retryable 503 (whose Retry-After relays untouched),
// the permanent 503, and core's own form-400 each cost exactly one
// upstream attempt on every leg. A retried login start would create a
// second server-side state; a retried ACS would spend the request
// twice — one forward is the whole exactly-once story dest-side.
func TestSAMLTerm_NoRetryOnUnstable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"core 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"saml login failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
		{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the session store is not configured"}`},
		{"acs refused 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"RelayState and SAMLResponse are required"}`},
	}
	for _, tc := range cases {
		core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		for _, route := range d9SAMLRoutes {
			path := samlTermParityPath(t, route)
			method, query, form := samlTermParityTarget(path)
			target := path
			if query != "" {
				target += "?" + query
			}
			var req *http.Request
			if form != "" {
				req = httptest.NewRequest(method, target, strings.NewReader(form))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest(method, target, nil)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Errorf("%s %s: upstream hits = %d, want exactly 1 (no dest retry)", tc.name, route, core.hits)
			}
			if rec.Code != tc.status {
				t.Errorf("%s %s: status = %d, want core status %d verbatim", tc.name, route, rec.Code, tc.status)
			}
			core.hits = 0
		}
	}
}

// TestSAMLTerm_DeadCoreSingleAttempt pins the dead-core shape: an
// unreachable core answers the permanent local 503
// backend_unavailable with no Retry-After, in both negotiated
// shapes, after exactly one attempt that fails inside the client.
func TestSAMLTerm_DeadCoreSingleAttempt(t *testing.T) {
	t.Parallel()

	// Dead port, never a recycled test-server URL: a closed httptest
	// server's port can be rebound by a parallel test, turning this
	// dead-core proof into a hit on a live stub (eshu-api#94).
	deadURL := "http://127.0.0.1:1"

	srv := NewServer(deadURL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, route := range d9SAMLRoutes {
		path := samlTermParityPath(t, route)
		method, _, form := samlTermParityTarget(path)
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			var req *http.Request
			if form != "" {
				req = httptest.NewRequest(method, path, strings.NewReader(form))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest(method, path, nil)
			}
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s Accept %q: status = %d, want permanent local 503", route, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%s Accept %q: Retry-After = %q, want empty (permanent, never retryable)", route, accept, got)
			}
		}
	}
}

// TestSAMLTerm_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: with no core origin configured all three terminated
// spellings answer the permanent 503 backend_unavailable in both
// negotiated shapes, with zero core contact to make.
func TestSAMLTerm_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, route := range d9SAMLRoutes {
		path := samlTermParityPath(t, route)
		method, _, form := samlTermParityTarget(path)
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			var req *http.Request
			if form != "" {
				req = httptest.NewRequest(method, path, strings.NewReader(form))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest(method, path, nil)
			}
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("%s Accept %q: panicked with %v, want 503", route, accept, recovered)
					}
				}()
				handler.ServeHTTP(rec, req)
			}()

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s Accept %q: status = %d, want the permanent 503", route, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%s Accept %q: Retry-After = %q, want empty (permanent, never retryable)", route, accept, got)
			}
		}
	}
}

// TestSAMLTerm_DisabledProviderDenyShape pins the disabled-provider
// answer: a core with no SAML provider configured refuses with its
// 503 unavailable shape, and the terminated metadata leg relays that
// shape verbatim instead of inventing a dest-side refusal or leaking
// provider detail.
func TestSAMLTerm_DisabledProviderDenyShape(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{status: http.StatusServiceUnavailable, body: `{"error":"Service Unavailable","detail":"saml login is unavailable"}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/metadata", nil)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("Accept %q: status = %d, want the disabled-provider 503", accept, rec.Code)
		}
		raw, _ := io.ReadAll(rec.Result().Body)
		if string(raw) != `{"error":"Service Unavailable","detail":"saml login is unavailable"}` {
			t.Errorf("Accept %q: body = %q, want the deny shape verbatim", accept, raw)
		}
		if core.hits != 1 {
			t.Errorf("Accept %q: upstream hits = %d, want exactly 1", accept, core.hits)
		}
	}
}

// TestSAMLTerm_UnknownProviderForwards pins the wildcard
// mux-collision contract for unknown provider IDs: the wildcard
// segment matches any single segment — including one core never
// configured — so dest forwards opaquely (upstream sees the unknown
// ID untouched) and relays core's 404 verbatim instead of inventing
// a dest-side refusal.
func TestSAMLTerm_UnknownProviderForwards(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusNotFound, body: `{"error":"Not Found","detail":"saml provider not found"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range []struct {
		method string
		path   string
		query  string
		body   string
	}{
		{http.MethodGet, "/api/v0/auth/saml/providers/never-configured-zzz/metadata", "", ""},
		{http.MethodGet, "/api/v0/auth/saml/providers/never-configured-zzz/login", "return_to=%2Fconsole", ""},
		{http.MethodPost, "/api/v0/auth/saml/providers/never-configured-zzz/acs", "", "RelayState=opaque-relay& SAMLResponse=opaque-response"},
	} {
		target := tc.path
		if tc.query != "" {
			target += "?" + tc.query
		}
		var req *http.Request
		if tc.body != "" {
			req = httptest.NewRequest(tc.method, target, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(tc.method, target, nil)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want core's unknown-provider 404 verbatim", tc.method, tc.path, rec.Code)
		}
		if core.lastPath != tc.path {
			t.Errorf("%s %s: upstream path = %q, want the unknown ID forwarded untouched", tc.method, tc.path, core.lastPath)
		}
	}
	if core.hits != 3 {
		t.Errorf("upstream hits = %d, want 3 (one forward per unknown-provider leg)", core.hits)
	}
}

// TestSAMLTerm_SiblingPathsStayRelayed pins the other half of the
// wildcard mux-collision contract: multi-segment provider spellings
// and deeper subtrees under a carved path match no wildcard route, so
// they fall through to the keyed generic relay and keep the standing
// 401 for credential-less callers with zero core contact.
func TestSAMLTerm_SiblingPathsStayRelayed(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"GET /api/v0/auth/saml/providers/a/b/login",
		"GET /api/v0/auth/saml/providers/a/b/metadata",
		"GET /api/v0/auth/saml/providers/p1/login/extra",
		"GET /api/v0/auth/saml/providers/p1/metadata/extra",
		"POST /api/v0/auth/saml/providers/p1/acs/extra",
	} {
		method, path, _ := strings.Cut(target, " ")
		req := httptest.NewRequest(method, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want the standing 401 (no wildcard match)", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (sibling paths never dispatch)", core.hits)
	}
}

// TestSAMLTerm_TerminatedFilesNoAuthorizer pins the telemetry
// extension without renames: the terminated SAML legs file their
// family on the existing native lane with the closed outcome
// vocabulary and time the port fetches — and file no authorizer
// judgment, because the routes are public and there is no gate to
// judge.
func TestSAMLTerm_TerminatedFilesNoAuthorizer(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	tel := NewTelemetry(nil)
	srv.WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	targets := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/metadata", ""},
		{http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", ""},
		{http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", "RelayState=opaque& SAMLResponse=opaque"},
	}
	for _, tc := range targets {
		var req *http.Request
		if tc.body != "" {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("%s %s status = %d, want 302", tc.method, tc.path, rec.Code)
		}
	}

	snap := tel.snapshot()
	if snap.dispatches["saml_login"]["native"]["core_ok"] != 3 {
		t.Errorf("dispatch rows = %v, want three saml_login/native/core_ok", snap.dispatches["saml_login"])
	}
	if _, ok := snap.authorizer["saml_login"]; ok {
		t.Errorf("authorizer rows = %v, want none for public saml_login (no gate to judge)", snap.authorizer["saml_login"])
	}
}
