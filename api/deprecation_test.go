// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The deprecation tests port the behavior pinned by
// deprecation_headers_test.go in go/cmd/api in eshu: /api/v0/*
// responses carry the horizon, /api/v1/* and non-API routes never do.

func TestDeprecationHeaders_PresentOnV0Routes(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	const sunset = "Thu, 01 Jul 2027 00:00:00 GMT"
	deprecated := deprecationHeadersMiddleware(mux, sunset)

	req := httptest.NewRequest("GET", "/api/v0/test", nil)
	rec := httptest.NewRecorder()
	deprecated.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v0/test status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Deprecation"); got != "true" {
		t.Fatalf("Deprecation header = %q, want 'true'", got)
	}
	if got := rec.Header().Get("Sunset"); got != sunset {
		t.Fatalf("Sunset header = %q, want %q", got, sunset)
	}
}

func TestDeprecationHeaders_AbsentOnV1Routes(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	const sunset = "Thu, 01 Jul 2027 00:00:00 GMT"
	// Full production layering: deprecation OUTER, v1 alias INNER.
	handler := deprecationHeadersMiddleware(v1PrefixAliasMiddleware(mux), sunset)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/test status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Fatalf("Deprecation header = %q, want empty on v1", got)
	}
	if got := rec.Header().Get("Sunset"); got != "" {
		t.Fatalf("Sunset header = %q, want empty on v1", got)
	}
}

func TestDeprecationHeaders_AbsentOnNonAPIRoutes(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	const sunset = "Thu, 01 Jul 2027 00:00:00 GMT"
	deprecated := deprecationHeadersMiddleware(mux, sunset)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	deprecated.ServeHTTP(rec, req)

	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Fatalf("Deprecation header = %q, want empty on non-API route", got)
	}
	if got := rec.Header().Get("Sunset"); got != "" {
		t.Fatalf("Sunset header = %q, want empty on non-API route", got)
	}
}

// TestDeprecationHeaders_V1AfterRewrite covers the full layering with a
// v1 request that rewrites to a v0 handler: the outer deprecation
// middleware saw the original /api/v1/* path, so no headers attach.
func TestDeprecationHeaders_V1AfterRewrite(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/repositories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	const sunset = "Thu, 01 Jul 2027 00:00:00 GMT"
	handler := deprecationHeadersMiddleware(v1PrefixAliasMiddleware(mux), sunset)

	t.Run("v1 request has no deprecation headers", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v1/repositories", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Deprecation"); got != "" {
			t.Fatalf("Deprecation header on v1 = %q, want empty", got)
		}
		if got := rec.Header().Get("Sunset"); got != "" {
			t.Fatalf("Sunset header on v1 = %q, want empty", got)
		}
	})

	t.Run("v0 request has deprecation headers", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v0/repositories", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Deprecation"); got != "true" {
			t.Fatalf("Deprecation header on v0 = %q, want 'true'", got)
		}
		if got := rec.Header().Get("Sunset"); got != sunset {
			t.Fatalf("Sunset header on v0 = %q, want %q", got, sunset)
		}
	})
}

// TestV0SunsetFromEnv pins the sunset override contract: a valid RFC
// 1123 value wins, anything else falls back to the default horizon.
func TestV0SunsetFromEnv(t *testing.T) {
	t.Parallel()

	custom := "Wed, 01 Jul 2026 00:00:00 GMT"
	if got := V0SunsetFromEnv(func(string) string { return custom }); got != custom {
		t.Errorf("V0SunsetFromEnv(valid) = %q, want %q", got, custom)
	}
	if got := V0SunsetFromEnv(func(string) string { return "" }); got != DefaultV0Sunset {
		t.Errorf("V0SunsetFromEnv(unset) = %q, want default %q", got, DefaultV0Sunset)
	}
	if got := V0SunsetFromEnv(func(string) string { return "next July" }); got != DefaultV0Sunset {
		t.Errorf("V0SunsetFromEnv(invalid) = %q, want default %q", got, DefaultV0Sunset)
	}
	if got := V0SunsetFromEnv(func(string) string { return "2027-07-01" }); got != DefaultV0Sunset {
		t.Errorf("V0SunsetFromEnv(non-RFC1123) = %q, want default %q", got, DefaultV0Sunset)
	}
}
