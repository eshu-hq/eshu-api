// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandlerServesHealthPins the binary surface: /health answers the
// contract versions and the configured origin, /readyz answers the probe
// shape, both without a credential, through the production chain.
func TestHandlerServesHealth(t *testing.T) {
	t.Parallel()

	srv := newServer("https://core:8443", "key")
	handler := srv.handler()

	for _, path := range []string{"/health", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read /health body: %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode /health body %q: %v", body, err)
	}
	if decoded["core_read_contracts"] != "v0" || decoded["admission_contracts"] != "v0" {
		t.Errorf("/health contracts = %v, want the pinned v0 versions", decoded)
	}
	if decoded["core"] != "https://core:8443" {
		t.Errorf("/health core = %q, want the configured origin", decoded["core"])
	}
}

// TestHandlerGatesAPIWithoutKey pins the fail-closed posture: with no
// ESHU_API_KEY in the environment shape (empty key here), /api/* denies
// even though the probes stay public.
func TestHandlerGatesAPIWithoutKey(t *testing.T) {
	t.Parallel()

	srv := newServer("https://core:8443", "")
	handler := srv.handler()

	req := httptest.NewRequest(http.MethodGet, "/api/v0/repositories", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v0/repositories with no key = %d, want 401", rec.Code)
	}
}

// TestRunRequiresCoreBaseURL pins the only required flag.
func TestRunRequiresCoreBaseURL(t *testing.T) {
	t.Parallel()

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := run([]string{}, quiet)
	if err == nil || !strings.Contains(err.Error(), "--core-base-url") {
		t.Fatalf("run() error = %v, want a missing --core-base-url error", err)
	}
}

// TestRunRejectsBadFlags pins flag parsing: an unknown flag fails the
// run instead of booting with a half-parsed configuration.
func TestRunRejectsBadFlags(t *testing.T) {
	t.Parallel()

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := run([]string{"--no-such-flag"}, quiet)
	if err == nil {
		t.Fatalf("run() error = nil, want a flag parse error")
	}
}
