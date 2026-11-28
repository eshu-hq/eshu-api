// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestHandlerServesHealth pins the binary surface: /health answers the
// contract versions and the configured origin regardless of core
// reachability, /readyz answers 200 with a live core and 503 with a
// dead one, all without a credential, through the production chain.
func TestHandlerServesHealth(t *testing.T) {
	t.Parallel()

	srv := newServer("https://core:8443", "key", slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler, err := srv.handler()
	if err != nil {
		t.Fatalf("handler() error = %v, want no error without a reader DSN", err)
	}

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
	liveHandler, err := liveSrv.handler()
	if err != nil {
		t.Fatalf("handler() error = %v, want no error without a reader DSN", err)
	}
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
	handler, err := srv.handler()
	if err != nil {
		t.Fatalf("handler() error = %v, want no error without a reader DSN", err)
	}

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

// TestHandlerReaderDSNUnsetKeepsUnconfiguredLane pins that the binary
// starts without the reader DSN: the work-items lane stays
// unconfigured (shared-key calls answer the local 503) and the boot
// succeeds.
func TestHandlerReaderDSNUnsetKeepsUnconfiguredLane(t *testing.T) {
	t.Setenv("ESHU_API_ADMIN_READ_DSN", "")

	srv := newServer("https://core:8443", "key", slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler, err := srv.handler()
	if err != nil {
		t.Fatalf("handler() error = %v, want no error without a reader DSN", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v0/admin/work-items/query", strings.NewReader(`{"limit":10}`))
	req.Header.Set("Authorization", "Bearer key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST work-items/query without a reader DSN = %d, want the local 503", rec.Code)
	}
}

// TestHandlerReaderDSNBadFailsBoot pins the fail-closed wiring: a
// mounted but unreachable reader DSN refuses the boot instead of
// serving a half-wired lane.
func TestHandlerReaderDSNBadFailsBoot(t *testing.T) {
	t.Setenv("ESHU_API_ADMIN_READ_DSN", "postgres://127.0.0.1:1/eshu?sslmode=disable")
	t.Setenv("ESHU_API_ADMIN_READ_MAX_CONNS", "")

	srv := newServer("https://core:8443", "key", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := srv.handler(); err == nil {
		t.Fatal("handler() error = nil, want a boot refusal on an unreachable reader DSN")
	}
}

// TestHandlerReaderDSNBadConfigFailsBoot pins the config leg: a
// mounted DSN with an unparsable pool ceiling refuses the boot at
// config load, before any dial.
func TestHandlerReaderDSNBadConfigFailsBoot(t *testing.T) {
	t.Setenv("ESHU_API_ADMIN_READ_DSN", "postgres://127.0.0.1:1/eshu?sslmode=disable")
	t.Setenv("ESHU_API_ADMIN_READ_MAX_CONNS", "many")

	srv := newServer("https://core:8443", "key", slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := srv.handler()
	if err == nil || !strings.Contains(err.Error(), "ESHU_API_ADMIN_READ_MAX_CONNS") {
		t.Fatalf("handler() error = %v, want a ceiling config refusal", err)
	}
}

// TestRunReaderDSNBadFailsBoot pins that the boot refusal surfaces
// through run: a set-but-bad reader DSN fails the process with the
// required flag present, instead of serving.
func TestRunReaderDSNBadFailsBoot(t *testing.T) {
	t.Setenv("ESHU_API_ADMIN_READ_DSN", "postgres://127.0.0.1:1/eshu?sslmode=disable")
	t.Setenv("ESHU_API_ADMIN_READ_MAX_CONNS", "")

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := run([]string{"--core-base-url", "https://core:8443"}, quiet)
	if err == nil {
		t.Fatal("run() error = nil, want a boot refusal on an unreachable reader DSN")
	}
}

// TestHandlerReaderDSNLiveServesLocally is the env-gated binary
// proof: with a scratch-database DSN mounted, the boot wires the
// reader store and a shared-key work-items query serves the live
// rows locally. It runs only when the fixture environment sets
// ESHU_TEST_PG_DSN; otherwise it skips, per the sessionstore
// fixture convention. The scratch database (never the shared
// fixture database) carries the table, and the test drops it.
func TestHandlerReaderDSNLiveServesLocally(t *testing.T) {
	base := strings.TrimSpace(os.Getenv("ESHU_TEST_PG_DSN"))
	if base == "" {
		t.Skip("ESHU_TEST_PG_DSN unset: binary reader proof needs a live database")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer admin.Close()

	scratch := fmt.Sprintf("adminread_binary_proof_%d_%d", os.Getpid(), time.Now().UTC().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+scratch); err != nil {
		t.Fatalf("create scratch database: %v", err)
	}
	defer func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+scratch+" WITH (FORCE)")
	}()
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatalf("fixture DSN is not a URL DSN: %v", err)
	}
	u.Path = "/" + scratch
	scratchPool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("open scratch pool: %v", err)
	}
	defer scratchPool.Close()
	if _, err := scratchPool.Exec(ctx, `CREATE TABLE fact_work_items (
    work_item_id TEXT PRIMARY KEY, scope_id TEXT NOT NULL,
    generation_id TEXT NOT NULL, stage TEXT NOT NULL, domain TEXT NOT NULL,
    status TEXT NOT NULL, attempt_count INTEGER NOT NULL DEFAULT 0,
    replay_attempt_baseline BIGINT NOT NULL DEFAULT 0, lease_owner TEXT NULL,
    failure_class TEXT NULL, failure_message TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
    visible_at TIMESTAMPTZ NULL, failure_details TEXT NULL)`); err != nil {
		t.Fatalf("create fact_work_items: %v", err)
	}
	if _, err := scratchPool.Exec(ctx, `INSERT INTO fact_work_items
    (work_item_id, scope_id, generation_id, stage, domain, status, created_at, updated_at)
    VALUES ('bw1', 's1', 'g1', 'build', 'd', 'pending',
     '2026-03-04T05:06:07Z', '2026-03-04T06:06:07Z')`); err != nil {
		t.Fatalf("seed fact_work_items: %v", err)
	}

	t.Setenv("ESHU_API_ADMIN_READ_DSN", u.String())
	t.Setenv("ESHU_API_ADMIN_READ_MAX_CONNS", "")

	srv := newServer("https://core:8443", "key", slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler, err := srv.handler()
	if err != nil {
		t.Fatalf("handler() error = %v, want the wired reader lane", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v0/admin/work-items/query", strings.NewReader(`{"limit":10}`))
	req.Header.Set("Authorization", "Bearer key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST work-items/query with a live reader = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"bw1"`) {
		t.Errorf("body = %s, want the live row", rec.Body.String())
	}
}
