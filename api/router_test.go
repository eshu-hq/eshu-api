// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
// health advertises the served contract versions, readyz answers the
// probe shape, and neither requires a credential.
func TestRouter_HealthAndReadyArePublic(t *testing.T) {
	t.Parallel()

	srv := NewServer("https://core:8443", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{"/health", "/readyz"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, rec.Code)
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
	if health["core_read_contracts"] != "v0" || health["admission_contracts"] != "v0" {
		t.Errorf("/health contracts = %v, want the pinned v0 versions", health)
	}
	if health["core"] != "https://core:8443" {
		t.Errorf("/health core = %q, want the configured origin", health["core"])
	}

	ready := func() map[string]string {
		req := httptest.NewRequest("GET", "/readyz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return decodeMap(t, rec)
	}()
	if ready["service"] != "eshu-api" || ready["probe"] != "readyz" || ready["status"] != "ok" {
		t.Errorf("/readyz = %v, want the service/probe/status shape", ready)
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
		decoded := decodeMap(t, rec)
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

	t.Run("v0 with credential reaches the skeleton 503", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest("GET", "/api/v0/repositories?limit=5"))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503 (no core dispatch yet)", rec.Code)
		}
		if got := rec.Header().Get("Deprecation"); got != "true" {
			t.Fatalf("Deprecation = %q, want 'true' on v0 errors", got)
		}
		decoded := decodeMap(t, rec)
		if decoded["error_code"] != "backend_unavailable" {
			t.Errorf("error_code = %q, want 'backend_unavailable'", decoded["error_code"])
		}
		if rec.Result().Header.Get("Retry-After") != "" {
			t.Errorf("Retry-After is set on the permanent skeleton 503: the outage is not transient")
		}
	})

	t.Run("v1 with credential rewrites before auth", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest("GET", "/api/v1/repositories"))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503 (v1 rewrote to v0 before the gate)", rec.Code)
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
	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
	errSection, ok := decoded["error"].(map[string]any)
	if !ok {
		t.Fatalf("body has no error section: %v", decoded)
	}
	if errSection["code"] != "unauthenticated" {
		t.Errorf("error.code = %v, want 'unauthenticated'", errSection["code"])
	}
}
