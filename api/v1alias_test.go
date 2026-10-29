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

// The v1 alias tests port the behavior pinned by v1_alias_test.go in
// go/cmd/api in eshu. The alias is a pure path rewrite ahead of auth, so
// any divergence here is a cutover defect, not a local choice.

func TestV1Alias_RewritesPathAndDispatches(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"v0","path":"` + r.URL.Path + `"}`))
	})
	mux.HandleFunc("POST /api/v0/submit", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(body)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := v1PrefixAliasMiddleware(mux)

	t.Run("GET alias returns same handler", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /api/v1/test status = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"version":"v0"`) {
			t.Fatalf("GET /api/v1/test body = %q, want v0 handler response", rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"path":"/api/v0/test"`) {
			t.Fatalf("handler saw path %q, want /api/v0/test", rec.Body.String())
		}
	})

	t.Run("POST alias preserves method and body", func(t *testing.T) {
		t.Parallel()

		body := strings.NewReader(`{"key":"value"}`)
		req := httptest.NewRequest("POST", "/api/v1/submit", body)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("POST /api/v1/submit status = %d, want 201", rec.Code)
		}
		if rec.Body.String() != `{"key":"value"}` {
			t.Fatalf("POST /api/v1/submit body = %q, want %q", rec.Body.String(), `{"key":"value"}`)
		}
	})

	t.Run("GET v0 still works", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v0/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /api/v0/test status = %d, want 200", rec.Code)
		}
	})

	t.Run("GET v1 bare prefix is 404", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v1/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET /api/v1/ status = %d, want 404", rec.Code)
		}
	})

	t.Run("GET health route unaffected", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /health status = %d, want 200", rec.Code)
		}
	})
}

func TestV1Alias_PreservesQueryParameters(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/echo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.RawQuery))
	})

	handler := v1PrefixAliasMiddleware(mux)

	req := httptest.NewRequest("GET", "/api/v1/echo?key=value&foo=bar", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "key=value&foo=bar" {
		t.Fatalf("query = %q, want 'key=value&foo=bar'", rec.Body.String())
	}
}

func TestV1Alias_PreservesHeaders(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/headers", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.Header.Get("X-Custom")))
	})

	handler := v1PrefixAliasMiddleware(mux)

	req := httptest.NewRequest("GET", "/api/v1/headers", nil)
	req.Header.Set("X-Custom", "hello-world")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "hello-world" {
		t.Fatalf("header value = %q, want 'hello-world'", rec.Body.String())
	}
}

// TestV1Alias_MiddlewareRewritesBeforeAuth verifies the v1 prefix alias
// runs before the auth gate so downstream classification sees the
// rewritten /api/v0/ path. This simulates the production layering order.
func TestV1Alias_MiddlewareRewritesBeforeAuth(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/repositories", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.Path))
	})

	authGate := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/v0/") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	handler := v1PrefixAliasMiddleware(authGate(mux))

	req := httptest.NewRequest("GET", "/api/v1/repositories", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/repositories status = %d, want 200 (rewrite must happen before auth)", rec.Code)
	}
	if rec.Body.String() != "/api/v0/repositories" {
		t.Fatalf("handler saw path = %q, want /api/v0/repositories", rec.Body.String())
	}
}

// TestV1Alias_WrongOrderFailsAuth documents the layering requirement: if
// the rewrite ran after the auth gate, v1 requests would 403.
func TestV1Alias_WrongOrderFailsAuth(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/repositories", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.Path))
	})

	authGate := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/v0/") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	handler := authGate(v1PrefixAliasMiddleware(mux))

	req := httptest.NewRequest("GET", "/api/v1/repositories", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("GET /api/v1/repositories status = %d, want 403 (this test proves auth-before-rewrite fails)", rec.Code)
	}
}
