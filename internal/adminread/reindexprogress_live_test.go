// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package adminread

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// TestAdminReadReindexProgressLiveProof is the end-to-end pool/reader
// proof for the reindex-progress lane: it builds a scratch database
// carrying the fleet watermark table, the per-repository watermark
// table, the full-generations table, and the scopes table, opens the
// reader store against it, and proves the three bounded progress
// reads (fleet idle-vs-stored, per-repository point read with absent
// scopes, per-scope stored-full plus active generation) plus the
// reader-role grant (SELECT-only on all four tables reads; writes
// refuse). It runs only when the fixture environment sets
// ESHU_TEST_PG_DSN; otherwise it skips, per the sessionstore
// fixture convention.
func TestAdminReadReindexProgressLiveProof(t *testing.T) {
	base := strings.TrimSpace(os.Getenv("ESHU_TEST_PG_DSN"))
	if base == "" {
		t.Skip("ESHU_TEST_PG_DSN unset: reader-pool proof needs a live database")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer admin.Close()

	scratch := fmt.Sprintf("adminread_reindex_proof_%d_%d", os.Getpid(), time.Now().UTC().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+scratch); err != nil {
		t.Fatalf("create scratch database: %v", err)
	}
	defer func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+scratch+" WITH (FORCE)")
	}()

	scratchDSN := withDatabase(t, base, scratch)
	schema := `
CREATE TABLE runtime_ingester_control (
    ingester TEXT PRIMARY KEY,
    reindex_request_status TEXT NOT NULL DEFAULT 'idle',
    reindex_request_requested_at TIMESTAMPTZ NULL
);
CREATE TABLE repository_reindex_requests (
    scope_id TEXT PRIMARY KEY,
    requested_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE ingestion_scopes (
    scope_id TEXT PRIMARY KEY,
    scope_kind TEXT NOT NULL,
    source_key TEXT NOT NULL,
    collector_kind TEXT NOT NULL,
    active_generation_id TEXT NULL
);
CREATE TABLE scope_generations (
    generation_id TEXT PRIMARY KEY,
    scope_id TEXT NOT NULL,
    is_delta BOOLEAN NOT NULL DEFAULT false,
    observed_at TIMESTAMPTZ NOT NULL,
    ingested_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL,
    activated_at TIMESTAMPTZ NULL
)`
	scratchPool, err := pgxpool.New(ctx, scratchDSN)
	if err != nil {
		t.Fatalf("open scratch pool: %v", err)
	}
	defer scratchPool.Close()
	if _, err := scratchPool.Exec(ctx, schema); err != nil {
		t.Fatalf("create progress tables: %v", err)
	}

	seed := `
INSERT INTO runtime_ingester_control (ingester, reindex_request_status, reindex_request_requested_at) VALUES
    ('repository', 'pending', '2026-03-04T05:06:07Z');
INSERT INTO repository_reindex_requests (scope_id, requested_at) VALUES
    ('s1', '2026-03-04T06:06:07Z'),
    ('s2', '2026-03-04T04:06:07Z');
INSERT INTO ingestion_scopes (scope_id, scope_kind, source_key, collector_kind, active_generation_id) VALUES
    ('s1', 'repository', 'repo-1', 'ck-a', 'g1'),
    ('s2', 'repository', 'repo-2', 'ck-a', 'g2'),
    ('s3', 'repository', 'repo-3', 'ck-a', NULL);
INSERT INTO scope_generations (generation_id, scope_id, is_delta, observed_at, ingested_at, status, activated_at) VALUES
    ('g1', 's1', false, '2026-03-04T04:06:07Z', '2026-03-04T04:06:07Z', 'active', '2026-03-04T04:06:08Z'),
    ('g0', 's1', false, '2026-03-04T03:06:07Z', '2026-03-04T03:06:07Z', 'superseded', NULL),
    ('g2', 's2', false, '2026-03-04T06:06:07Z', '2026-03-04T06:06:07Z', 'active', '2026-03-04T06:06:08Z')`
	if _, err := scratchPool.Exec(ctx, seed); err != nil {
		t.Fatalf("seed progress tables: %v", err)
	}

	store, err := NewStore(ctx, Config{DSN: scratchDSN, MaxConns: 2})
	if err != nil {
		t.Fatalf("open reader store: %v", err)
	}
	defer store.Close()

	// Fleet watermark: the stored instant, not the zero time.
	fleet, err := store.GetReindexState(ctx, "repository")
	if err != nil {
		t.Fatalf("fleet read: %v", err)
	}
	if want := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC); !fleet.RequestedAt.Equal(want) {
		t.Errorf("fleet requested_at = %v, want %v", fleet.RequestedAt, want)
	}
	// Unknown ingester: idle with zero watermark, never an error.
	idle, err := store.GetReindexState(ctx, "never-recorded")
	if err != nil {
		t.Fatalf("idle fleet read: %v", err)
	}
	if !idle.RequestedAt.IsZero() {
		t.Errorf("idle fleet requested_at = %v, want zero", idle.RequestedAt)
	}

	// Per-repository point read: stored rows keyed by scope, absent
	// scopes missing so the caller falls back to the fleet
	// watermark.
	perRepo, err := store.RepositoryReindexRequestedAt(ctx, []string{"s1", "s3"})
	if err != nil {
		t.Fatalf("per-repository read: %v", err)
	}
	if len(perRepo) != 1 {
		t.Fatalf("per-repository rows = %v, want only s1 (s3 has no row)", perRepo)
	}
	if want := time.Date(2026, 3, 4, 6, 6, 7, 0, time.UTC); !perRepo["s1"].Equal(want) {
		t.Errorf("s1 watermark = %v, want %v", perRepo["s1"], want)
	}

	// Per-scope state: s1 has a stored full (the superseded
	// pending g0 never counts) plus the active generation; s3 has
	// neither.
	s1, err := store.FullReconcileState(ctx, "s1")
	if err != nil {
		t.Fatalf("s1 scope read: %v", err)
	}
	if !s1.HasProjectedFull {
		t.Error("s1 has_projected_full = false, want true (activated g1)")
	}
	if want := time.Date(2026, 3, 4, 4, 6, 7, 0, time.UTC); !s1.LastStoredFullAt.Equal(want) {
		t.Errorf("s1 last_stored_full_at = %v, want %v (g1, not the re-projected time)", s1.LastStoredFullAt, want)
	}
	if s1.ActiveGenerationID != "g1" {
		t.Errorf("s1 active_generation_id = %q, want g1", s1.ActiveGenerationID)
	}
	s3, err := store.FullReconcileState(ctx, "s3")
	if err != nil {
		t.Fatalf("s3 scope read: %v", err)
	}
	if s3.HasProjectedFull || !s3.LastStoredFullAt.IsZero() || s3.ActiveGenerationID != "" {
		t.Errorf("s3 state = %+v, want the zero state (never seen)", s3)
	}
	blank, err := store.FullReconcileState(ctx, "  ")
	if err != nil {
		t.Fatalf("blank scope read: %v", err)
	}
	if blank != (apilibadmin.FullReconcileState{}) {
		t.Errorf("blank scope state = %+v, want zero without a query", blank)
	}

	// A canceled context fails the read: the store honors the
	// caller's bound instead of running unbounded.
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := store.GetReindexState(canceled, "repository"); err == nil {
		t.Error("canceled context: want the failure, got nil")
	}

	// Reader-role grant proof: a role holding only USAGE + SELECT
	// on all four tables reads through the same three query texts,
	// and any write refuses.
	role := fmt.Sprintf("adminread_reindex_proof_reader_%d", os.Getpid())
	if _, err := scratchPool.Exec(ctx, "CREATE ROLE "+role+" WITH NOLOGIN"); err != nil {
		t.Fatalf("create proof role: %v", err)
	}
	defer func() {
		for _, cleanup := range []string{
			"REVOKE ALL ON runtime_ingester_control FROM " + role,
			"REVOKE ALL ON repository_reindex_requests FROM " + role,
			"REVOKE ALL ON scope_generations FROM " + role,
			"REVOKE ALL ON ingestion_scopes FROM " + role,
			"REVOKE ALL ON SCHEMA public FROM " + role,
			"REVOKE " + role + " FROM CURRENT_USER",
			"DROP ROLE " + role,
		} {
			if _, err := scratchPool.Exec(ctx, cleanup); err != nil {
				t.Logf("role cleanup %q: %v", cleanup, err)
			}
		}
	}()
	for _, grant := range []string{
		"GRANT USAGE ON SCHEMA public TO " + role,
		"GRANT SELECT ON runtime_ingester_control TO " + role,
		"GRANT SELECT ON repository_reindex_requests TO " + role,
		"GRANT SELECT ON scope_generations TO " + role,
		"GRANT SELECT ON ingestion_scopes TO " + role,
		"GRANT " + role + " TO CURRENT_USER",
	} {
		if _, err := scratchPool.Exec(ctx, grant); err != nil {
			t.Fatalf("grant %q: %v", grant, err)
		}
	}
	rolePool, err := pgxpool.New(ctx, scratchDSN)
	if err != nil {
		t.Fatalf("open role pool: %v", err)
	}
	defer rolePool.Close()
	conn, err := rolePool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire role conn: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SET ROLE "+role); err != nil {
		t.Fatalf("assume proof role: %v", err)
	}
	roleFleetQuery, roleFleetArgs := buildGetReindexStateQuery("repository")
	roleFleetRows, err := conn.Query(ctx, roleFleetQuery, roleFleetArgs...)
	if err != nil {
		t.Fatalf("reader-role fleet read: %v", err)
	}
	roleFleet, err := scanReindexState(roleFleetRows)
	roleFleetRows.Close()
	if err != nil {
		t.Fatalf("reader-role fleet scan: %v", err)
	}
	if want := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC); !roleFleet.Equal(want) {
		t.Errorf("reader-role fleet = %v, want %v (SELECT-only suffices)", roleFleet, want)
	}
	roleRepoQuery, roleRepoArgs := buildRepositoryReindexRequestedAtQuery([]string{"s1", "s3"})
	roleRepoRows, err := conn.Query(ctx, roleRepoQuery, roleRepoArgs...)
	if err != nil {
		t.Fatalf("reader-role per-repository read: %v", err)
	}
	roleRepo, err := scanRepositoryWatermarks(roleRepoRows)
	roleRepoRows.Close()
	if err != nil {
		t.Fatalf("reader-role per-repository scan: %v", err)
	}
	if len(roleRepo) != 1 {
		t.Errorf("reader-role per-repository rows = %v, want only s1", roleRepo)
	}
	roleScopeQuery, roleScopeArgs := buildFullReconcileStateQuery("s1")
	roleScopeRows, err := conn.Query(ctx, roleScopeQuery, roleScopeArgs...)
	if err != nil {
		t.Fatalf("reader-role scope read: %v", err)
	}
	roleScope, err := scanFullReconcileState(roleScopeRows)
	roleScopeRows.Close()
	if err != nil {
		t.Fatalf("reader-role scope scan: %v", err)
	}
	if !roleScope.HasProjectedFull || roleScope.ActiveGenerationID != "g1" {
		t.Errorf("reader-role scope = %+v, want the stored full plus g1", roleScope)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO repository_reindex_requests (scope_id) VALUES ('wx')"); err == nil {
		t.Error("reader-role write succeeded, want a permission refusal")
	}
}
