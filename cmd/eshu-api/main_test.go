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
	"time"
)

// TestHandlerServesHealth pins the binary surface: /health answers the
// contract versions and the configured origin regardless of core
// reachability, /readyz answers 200 with a live core and 503 with a
// dead one, all without a credential, through the production chain.
func TestHandlerServesHealth(t *testing.T) {
	t.Parallel()

	srv := newServer("https://core:8443", "key", slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := srv.handler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health = %d, want 200 with a dead core", rec.Code)
	}
	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read /health body: %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode /health body %q: %v", body, err)
	}
	if decoded["core_read_contracts"] != "v0" || decoded["suppression_admission_contracts"] != "v0" {
		t.Errorf("/health contracts = %v, want the pinned v0 versions", decoded)
	}
	if decoded["core"] != "https://core:8443" {
		t.Errorf("/health core = %q, want the configured origin", decoded["core"])
	}

	readyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyRec := httptest.NewRecorder()
	handler.ServeHTTP(readyRec, readyReq)
	if readyRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz with a dead core = %d, want 503", readyRec.Code)
	}

	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer live.Close()
	liveSrv := newServer(live.URL, "key", slog.New(slog.NewTextHandler(io.Discard, nil)))
	liveHandler := liveSrv.handler()
	liveReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	liveRec := httptest.NewRecorder()
	liveHandler.ServeHTTP(liveRec, liveReq)
	if liveRec.Code != http.StatusOK {
		t.Fatalf("GET /readyz with a live core = %d, want 200", liveRec.Code)
	}
}

// TestBuildServerSetsTimeouts pins the production server bounds: every
// connection timeout is set, so no slow client can hold a worker
// indefinitely, and the write bound clears the 30 s relay budget.
func TestBuildServerSetsTimeouts(t *testing.T) {
	t.Parallel()

	srv := buildServer(":0", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	if srv.ReadHeaderTimeout != serverReadHeaderTimeout || serverReadHeaderTimeout <= 0 {
		t.Errorf("ReadHeaderTimeout = %v, want the positive pinned bound", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != serverReadTimeout || serverReadTimeout <= 0 {
		t.Errorf("ReadTimeout = %v, want the positive pinned bound", srv.ReadTimeout)
	}
	if srv.WriteTimeout != serverWriteTimeout || serverWriteTimeout <= 30*time.Second {
		t.Errorf("WriteTimeout = %v, want a bound clearing the 30 s relay budget", srv.WriteTimeout)
	}
	if srv.IdleTimeout != serverIdleTimeout || serverIdleTimeout <= 0 {
		t.Errorf("IdleTimeout = %v, want the positive pinned bound", srv.IdleTimeout)
	}
}

// TestHandlerGatesAPIWithoutKey pins the fail-closed posture: with no
// ESHU_API_KEY in the environment shape (empty key here), /api/* denies
// even though the probes stay public.
func TestHandlerGatesAPIWithoutKey(t *testing.T) {
	t.Parallel()

	srv := newServer("https://core:8443", "", slog.New(slog.NewTextHandler(io.Discard, nil)))
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
