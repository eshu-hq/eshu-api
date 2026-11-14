// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// decodeMap decodes a recorder body into a string-keyed map.
func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()

	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
	return decoded
}

// authedRequest builds a request carrying the exact skeleton test key.
func authedRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	return req
}

// TestRouter_HealthAndReadyArePublic pins the unauthenticated probes:
// health advertises the served contract versions regardless of core
// reachability, readyz reflects it (200 with a live core, 503 with a
// dead or unconfigured one), and neither requires a credential.
func TestRouter_HealthAndReadyArePublic(t *testing.T) {
	t.Parallel()

	srv := NewServer("https://core:8443", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{"/health", "/readyz"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if path == "/health" && rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200 (health never reflects reachability)", path, rec.Code)
		}
		if got := rec.Header().Get("Deprecation"); got != "" {
			t.Fatalf("GET %s Deprecation = %q, want empty on probes", path, got)
		}
	}

	health := func() map[string]string {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return decodeMap(t, rec)
	}()
	if health["status"] != "ok" || health["service"] != "eshu-api" {
		t.Errorf("/health = %v, want status ok for eshu-api", health)
	}
	if health["core_read_contracts"] != "v0" || health["suppression_admission_contracts"] != "v0" {
		t.Errorf("/health contracts = %v, want the pinned v0 versions", health)
	}
	for _, key := range []string{
		"suppression_admission_contracts",
		"terminal_failure_admission_contracts",
		"replay_ledger_admission_contracts",
		"sign_in_policy_admission_contracts",
		"retirement_admission_contracts",
		"admin_identity_admission_contracts",
		"setup_admission_contracts",
		"provider_config_mutation_admission_contracts",
		"provider_config_read_admission_contracts",
		"oidc_login_admission_contracts",
		"github_login_admission_contracts",
		"saml_login_admission_contracts",
		"admin_read_admission_contracts",
		"admin_identity_read_admission_contracts",
		"governance_audit_read_admission_contracts",
		"terminal_failure_read_admission_contracts",
	} {
		if health[key] != "v0" {
			t.Errorf("/health %s = %q, want the pinned v0 version", key, health[key])
		}
	}
	if health["core"] != "https://core:8443" {
		t.Errorf("/health core = %q, want the configured origin", health["core"])
	}

	// With a dead core origin the probe reads not-ready: 503 with the
	// closed failure reason, while /health above stayed 200.
	readyNotReady := func() map[string]any {
		req := httptest.NewRequest("GET", "/readyz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("GET /readyz with a dead core = %d, want 503", rec.Code)
		}
		return decodeBody(t, rec)
	}()
	if readyNotReady["status"] != "unavailable" {
		t.Errorf("/readyz = %v, want status unavailable with a dead core", readyNotReady)
	}
	readyDetails, _ := readyNotReady["details"].(map[string]any)
	if readyDetails["reason"] == nil || readyDetails["reason"] == "" {
		t.Errorf("/readyz = %v, want a closed details.reason with a dead core", readyNotReady)
	}

	// With a live core the probe keeps the historical 200 shape.
	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer live.Close()
	liveReady := func() map[string]string {
		req := httptest.NewRequest("GET", "/readyz", nil)
		rec := httptest.NewRecorder()
		NewServer(live.URL, skeletonTestKey).Handler(DefaultV0Sunset).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /readyz with a live core = %d, want 200", rec.Code)
		}
		return decodeMap(t, rec)
	}()
	if liveReady["service"] != "eshu-api" || liveReady["probe"] != "readyz" || liveReady["status"] != "ok" {
		t.Errorf("/readyz = %v, want the service/probe/status shape", liveReady)
	}
}

// TestRouter_FullChain pins the production layering end to end on the
// skeleton surface: unauthenticated /api/v0/* denies with the v0
// deprecation headers attached; the same route over /api/v1/* denies
// without them; an authenticated call reaches the explicit 503 (proving
// the v1 rewrite ran before auth); unknown paths 404.
func TestRouter_FullChain(t *testing.T) {
	t.Parallel()

	srv := NewServer("https://core:8443", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	t.Run("v0 without credential denies with deprecation headers", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v0/repositories", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if got := rec.Header().Get("Deprecation"); got != "true" {
			t.Fatalf("Deprecation = %q, want 'true' on v0 errors", got)
		}
		if got := rec.Header().Get("Sunset"); got != DefaultV0Sunset {
			t.Fatalf("Sunset = %q, want the default horizon", got)
		}
		decoded := decodeBody(t, rec)
		if decoded["error_code"] != "unauthenticated" {
			t.Errorf("error_code = %q, want 'unauthenticated'", decoded["error_code"])
		}
	})

	t.Run("v1 without credential denies with no deprecation headers", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v1/repositories", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if got := rec.Header().Get("Deprecation"); got != "" {
			t.Fatalf("Deprecation = %q, want empty on v1", got)
		}
		if got := rec.Header().Get("Sunset"); got != "" {
			t.Fatalf("Sunset = %q, want empty on v1", got)
		}
	})

	t.Run("v0 with credential dispatches to the core", func(t *testing.T) {
		t.Parallel()

		core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest("GET", "/api/v0/repositories?limit=5"))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want the core's 200", rec.Code)
		}
		if core.lastPath != "/api/v0/repositories" || core.lastQuery != "limit=5" {
			t.Errorf("upstream saw %s?%s, want /api/v0/repositories?limit=5", core.lastPath, core.lastQuery)
		}
		if got := rec.Header().Get("Deprecation"); got != "true" {
			t.Fatalf("Deprecation = %q, want 'true' on v0 dispatch", got)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if string(body) != core.body {
			t.Errorf("body = %q, want the core bytes %q", body, core.body)
		}
	})

	t.Run("v1 with credential rewrites before auth", func(t *testing.T) {
		t.Parallel()

		core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest("GET", "/api/v1/repositories"))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want the core's 200 (v1 rewrote to v0 before the gate)", rec.Code)
		}
		if core.lastPath != "/api/v0/repositories" {
			t.Errorf("upstream path = %q, want the rewritten v0 path", core.lastPath)
		}
		if got := rec.Header().Get("Deprecation"); got != "" {
			t.Fatalf("Deprecation = %q, want empty on v1 even after the rewrite", got)
		}
	})

	t.Run("unknown path is 404", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/nope", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
}

// TestRouter_EnvelopeNegotiation pins the envelope 401 through the full
// chain: an envelope caller gets the envelope error section.
func TestRouter_EnvelopeNegotiation(t *testing.T) {
	t.Parallel()

	srv := NewServer("https://core:8443", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest("GET", "/api/v0/repositories", nil)
	req.Header.Set("Accept", "application/eshu.envelope+json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	decoded, keys := decodeEnvelope(t, rec)
	if !slices.Equal(keys, []string{"data", "error", "truth"}) {
		t.Fatalf("top-level keys = %v, want [data error truth]", keys)
	}
	errSection, ok := decoded["error"].(map[string]any)
	if !ok {
		t.Fatalf("body has no error section: %v", decoded)
	}
	if errSection["code"] != "unauthenticated" {
		t.Errorf("error.code = %v, want 'unauthenticated'", errSection["code"])
	}
}

// TestRouter_EnvelopeDispatchRelays pins the negotiated-envelope core
// route: an authenticated envelope caller's Accept crosses to the core
// and the core's envelope bytes come back untouched, with the v0
// deprecation headers from the outer middleware.
func TestRouter_EnvelopeDispatchRelays(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusOK,
		body:   `{"data":{"ok":true},"error":null,"truth":null}`,
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest("GET", "/api/v0/repositories")
	req.Header.Set("Accept", "application/eshu.envelope+json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want the core's 200", rec.Code)
	}
	if core.lastAccept != "application/eshu.envelope+json" {
		t.Errorf("upstream Accept = %q, want the caller's envelope MIME", core.lastAccept)
	}
	if got := rec.Header().Get("Deprecation"); got != "true" {
		t.Fatalf("Deprecation = %q, want 'true' on v0 dispatch", got)
	}
	decoded, keys := decodeEnvelope(t, rec)
	if !slices.Equal(keys, []string{"data", "error", "truth"}) {
		t.Fatalf("top-level keys = %v, want [data error truth]", keys)
	}
	if decoded["data"] == nil {
		t.Errorf("envelope data = nil, want the core's payload relayed")
	}
}
