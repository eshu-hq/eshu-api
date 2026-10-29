// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const skeletonTestKey = "test-shared-key"

// decodeBody decodes a recorder body into a string map for shape assertions.
func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
	return decoded
}

// TestWriteUnauthorized_PlainShape pins the default 401 wire shape: the
// bare Bearer [REDACTED] the error_code/message/correlation_id body the live
// path serves today to callers that did not negotiate the envelope.
func TestWriteUnauthorized_PlainShape(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/api/v0/status", nil)
	rec := httptest.NewRecorder()
	WriteUnauthorized(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want bare 'Bearer'", got)
	}
	decoded := decodeBody(t, rec)
	if decoded["error_code"] != "unauthenticated" {
		t.Errorf("error_code = %v, want 'unauthenticated'", decoded["error_code"])
	}
	if decoded["message"] != "authentication is required" {
		t.Errorf("message = %v, want 'authentication is required'", decoded["message"])
	}
	correlation, _ := decoded["correlation_id"].(string)
	if correlation == "" {
		t.Errorf("correlation_id is empty, want a generated identifier")
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}
}

// TestWriteUnauthorized_EnvelopeShape pins the negotiated-envelope 401:
// the error section carries code/message/correlation_id.
func TestWriteUnauthorized_EnvelopeShape(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/api/v0/status", nil)
	req.Header.Set("Accept", "application/eshu.envelope+json")
	rec := httptest.NewRecorder()
	WriteUnauthorized(rec, req)

	if rec.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Result().StatusCode)
	}
	decoded := decodeBody(t, rec)
	errSection, ok := decoded["error"].(map[string]any)
	if !ok {
		t.Fatalf("body has no error section: %v", decoded)
	}
	if errSection["code"] != "unauthenticated" {
		t.Errorf("error.code = %v, want 'unauthenticated'", errSection["code"])
	}
	if errSection["message"] != "authentication is required" {
		t.Errorf("error.message = %v, want 'authentication is required'", errSection["message"])
	}
	if errSection["correlation_id"] == nil || errSection["correlation_id"] == "" {
		t.Errorf("error.correlation_id is empty, want a generated identifier")
	}
}

// TestCorrelationID_EchoesCallerHeaders pins correlation propagation: a
// caller-supplied ID round-trips, otherwise the server mints one. Two
// minted IDs must differ so they can actually correlate distinct calls.
func TestCorrelationID_EchoesCallerHeaders(t *testing.T) {
	t.Parallel()

	echo := func(header, value string) string {
		req := httptest.NewRequest("GET", "/api/v0/status", nil)
		if header != "" {
			req.Header.Set(header, value)
		}
		rec := httptest.NewRecorder()
		WriteUnauthorized(rec, req)
		decoded := decodeBody(t, rec)
		correlation, _ := decoded["correlation_id"].(string)
		return correlation
	}

	if got := echo("X-Correlation-ID", "caller-1"); got != "caller-1" {
		t.Errorf("X-Correlation-ID echo = %q, want 'caller-1'", got)
	}
	if got := echo("X-Request-ID", "caller-2"); got != "caller-2" {
		t.Errorf("X-Request-ID echo = %q, want 'caller-2'", got)
	}
	first, second := echo("", ""), echo("", "")
	if first == "" || second == "" {
		t.Fatalf("minted IDs = %q, %q: want non-empty identifiers", first, second)
	}
	if first == second {
		t.Errorf("minted IDs are equal (%q): want distinct identifiers", first)
	}
}

// TestRequireAuth_Gate pins the gate: public probes pass without a key,
// /api/* passes only with the exact bearer key, everything else 401s.
// An empty configured key fails closed.
func TestRequireAuth_Gate(t *testing.T) {
	t.Parallel()

	downstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	call := func(apiKey, path, authHeader string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		RequireAuth(apiKey, downstream).ServeHTTP(rec, req)
		return rec
	}

	t.Run("health passes without a key", func(t *testing.T) {
		t.Parallel()
		if rec := call("", "/health", ""); rec.Code != http.StatusOK {
			t.Fatalf("GET /health status = %d, want 200", rec.Code)
		}
	})

	t.Run("readyz passes without a key", func(t *testing.T) {
		t.Parallel()
		if rec := call("", "/readyz", ""); rec.Code != http.StatusOK {
			t.Fatalf("GET /readyz status = %d, want 200", rec.Code)
		}
	})

	t.Run("api route without credential is 401", func(t *testing.T) {
		t.Parallel()

		rec := call(skeletonTestKey, "/api/v0/repositories", "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if got := rec.Result().Header.Get("WWW-Authenticate"); got != "Bearer" {
			t.Fatalf("WWW-Authenticate = %q, want bare 'Bearer'", got)
		}
	})

	t.Run("api route with wrong key is 401", func(t *testing.T) {
		t.Parallel()

		rec := call(skeletonTestKey, "/api/v0/repositories", "Bearer wrong")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("api route with non-bearer scheme is 401", func(t *testing.T) {
		t.Parallel()

		rec := call(skeletonTestKey, "/api/v0/repositories", "Basic abc123")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("api route with empty bearer is 401", func(t *testing.T) {
		t.Parallel()

		rec := call(skeletonTestKey, "/api/v0/repositories", "Bearer ")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("api route with exact key passes", func(t *testing.T) {
		t.Parallel()

		rec := call(skeletonTestKey, "/api/v0/repositories", "Bearer "+skeletonTestKey)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("empty configured key fails closed", func(t *testing.T) {
		t.Parallel()

		rec := call("", "/api/v0/repositories", "Bearer "+skeletonTestKey)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401 with no key configured", rec.Code)
		}
	})

	t.Run("key comparison is exact", func(t *testing.T) {
		t.Parallel()

		for _, candidate := range []string{"Bearer " + skeletonTestKey + " ", "Bearer  " + skeletonTestKey} {
			rec := call(skeletonTestKey, "/api/v0/repositories", candidate)
			if rec.Code != http.StatusOK {
				t.Fatalf("auth %q status = %d, want 200 (surrounding whitespace is trimmed)", candidate, rec.Code)
			}
		}
		if rec := call(skeletonTestKey, "/api/v0/repositories", "Bearer "+strings.ToUpper(skeletonTestKey)); rec.Code != http.StatusUnauthorized {
			t.Fatalf("uppercased key status = %d, want 401 (comparison is case-sensitive)", rec.Code)
		}
	})
}
