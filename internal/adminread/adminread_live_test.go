// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package adminread

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// TestAdminReadLiveProof is the end-to-end pool/reader proof for the
// work-items lane: it builds a scratch database carrying a
// fact_work_items table with the real core column types, opens the
// reader store against it, and proves the bounded list (ordering,
// filters, default limit, details fold) plus the reader-role grant
// (SELECT-only role reads; writes refuse). It runs only when the
// fixture environment sets ESHU_TEST_PG_DSN; otherwise it skips, per
// the sessionstore fixture convention.
func TestAdminReadLiveProof(t *testing.T) {
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

	scratch := fmt.Sprintf("adminread_proof_%d_%d", os.Getpid(), time.Now().UTC().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+scratch); err != nil {
		t.Fatalf("create scratch database: %v", err)
	}
	defer func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+scratch+" WITH (FORCE)")
	}()

	scratchDSN := withDatabase(t, base, scratch)
	schema := `
CREATE TABLE fact_work_items (
    work_item_id TEXT PRIMARY KEY,
    scope_id TEXT NOT NULL,
    generation_id TEXT NOT NULL,
    stage TEXT NOT NULL,
    domain TEXT NOT NULL,
    status TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    replay_attempt_baseline BIGINT NOT NULL DEFAULT 0,
    lease_owner TEXT NULL,
    failure_class TEXT NULL,
    failure_message TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    visible_at TIMESTAMPTZ NULL,
    failure_details TEXT NULL
)`
	scratchPool, err := pgxpool.New(ctx, scratchDSN)
	if err != nil {
		t.Fatalf("open scratch pool: %v", err)
	}
	defer scratchPool.Close()
	if _, err := scratchPool.Exec(ctx, schema); err != nil {
		t.Fatalf("create fact_work_items: %v", err)
	}

	seed := `
INSERT INTO fact_work_items
    (work_item_id, scope_id, generation_id, stage, domain, status,
     attempt_count, replay_attempt_baseline, lease_owner, failure_class,
     created_at, updated_at, visible_at, failure_details)
VALUES
    ('w1', 's1', 'g1', 'build', 'd', 'pending', 3, 1, 'owner-1', 'build_failed',
     '2026-03-04T05:06:07Z', '2026-03-04T08:06:07Z', '2026-03-04T07:06:07Z',
     '{"operator_note":"look again","prior_failure":{"status":"failed","failure_class":"build_failed","failure_message":"boom","updated_at":"2026-03-04T05:06:07.5Z"}}'),
    ('w2', 's1', 'g1', 'build', 'd', 'failed', 1, 0, NULL, NULL,
     '2026-03-04T05:06:07Z', '2026-03-04T06:06:07Z', NULL, NULL),
    ('w3', 's2', 'g2', 'test', 'd', 'pending', 0, 0, NULL, NULL,
     '2026-03-04T05:06:07Z', '2026-03-04T04:06:07Z', NULL, 'not json')`
	if _, err := scratchPool.Exec(ctx, seed); err != nil {
		t.Fatalf("seed fact_work_items: %v", err)
	}

	store, err := NewStore(ctx, Config{DSN: scratchDSN, MaxConns: 2})
	if err != nil {
		t.Fatalf("open reader store: %v", err)
	}
	defer store.Close()

	items, err := store.ListWorkItems(ctx, apilibadmin.WorkItemFilter{})
	if err != nil {
		t.Fatalf("bounded list: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("bounded list rows = %d, want 3", len(items))
	}
	if items[0].WorkItemID != "w1" || items[1].WorkItemID != "w2" || items[2].WorkItemID != "w3" {
		t.Fatalf("bounded list order = %v, want updated_at DESC (w1 w2 w3)", rowIDs(items))
	}
	first := items[0]
	if first.OperatorNote == nil || *first.OperatorNote != "look again" {
		t.Errorf("w1 operator_note = %v, want the unfolded note", first.OperatorNote)
	}
	if first.PriorFailure == nil || first.PriorFailure.UpdatedAt != "2026-03-04T05:06:07Z" {
		t.Errorf("w1 prior_failure = %+v, want the kept failure with RFC3339 instant", first.PriorFailure)
	}
	if items[2].OperatorNote != nil || items[2].PriorFailure != nil {
		t.Errorf("w3 non-JSON details unfolded: %+v", items[2])
	}

	filtered, err := store.ListWorkItems(ctx, apilibadmin.WorkItemFilter{Statuses: []string{"failed"}})
	if err != nil {
		t.Fatalf("status filter: %v", err)
	}
	if len(filtered) != 1 || filtered[0].WorkItemID != "w2" {
		t.Errorf("status filter rows = %v, want [w2]", rowIDs(filtered))
	}
	scoped, err := store.ListWorkItems(ctx, apilibadmin.WorkItemFilter{ScopeID: "  s2  "})
	if err != nil {
		t.Fatalf("scope filter: %v", err)
	}
	if len(scoped) != 1 || scoped[0].WorkItemID != "w3" {
		t.Errorf("scope filter rows = %v, want [w3]", rowIDs(scoped))
	}
	paged, err := store.ListWorkItems(ctx, apilibadmin.WorkItemFilter{Limit: 1})
	if err != nil {
		t.Fatalf("limit 1: %v", err)
	}
	if len(paged) != 1 || paged[0].WorkItemID != "w1" {
		t.Errorf("limit 1 rows = %v, want [w1]", rowIDs(paged))
	}

	// Reader-role grant proof: a role holding only USAGE + SELECT
	// reads through the same query text, and any write refuses.
	role := fmt.Sprintf("adminread_proof_reader_%d", os.Getpid())
	if _, err := scratchPool.Exec(ctx, "CREATE ROLE "+role+" WITH NOLOGIN"); err != nil {
		t.Fatalf("create proof role: %v", err)
	}
	defer func() {
		// Privilege dependencies block DROP ROLE: revoke the
		// proof grants (and the test membership) before dropping.
		for _, cleanup := range []string{
			"REVOKE ALL ON fact_work_items FROM " + role,
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
		"GRANT SELECT ON fact_work_items TO " + role,
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
	query, args := buildListWorkItemsQuery(apilibadmin.WorkItemFilter{})
	pgRows, err := conn.Query(ctx, query, args...)
	if err != nil {
		t.Fatalf("reader-role list: %v", err)
	}
	roleItems, err := scanWorkItems(pgRows)
	pgRows.Close()
	if err != nil {
		t.Fatalf("reader-role scan: %v", err)
	}
	if len(roleItems) != 3 {
		t.Errorf("reader-role rows = %d, want 3 (SELECT-only suffices)", len(roleItems))
	}
	if _, err := conn.Exec(ctx, "INSERT INTO fact_work_items (work_item_id) VALUES ('wx')"); err == nil {
		t.Error("reader-role write succeeded, want a permission refusal")
	}
}

// TestAdminReadDeadLettersLiveProof is the end-to-end pool/reader
// proof for the dead-letters lane: it builds a scratch database
// carrying fact_work_items plus the joined ingestion_scopes, opens
// the reader store against it, and proves the bounded dead-letter
// list (status predicate, ordering, filters, limit) plus the
// reader-role grant (SELECT-only on both tables reads, including
// across the join; writes refuse). It runs only when the fixture
// environment sets ESHU_TEST_PG_DSN; otherwise it skips, per the
// sessionstore fixture convention.
func TestAdminReadDeadLettersLiveProof(t *testing.T) {
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

	scratch := fmt.Sprintf("adminread_deadletters_proof_%d_%d", os.Getpid(), time.Now().UTC().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+scratch); err != nil {
		t.Fatalf("create scratch database: %v", err)
	}
	defer func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+scratch+" WITH (FORCE)")
	}()

	scratchDSN := withDatabase(t, base, scratch)
	schema := `
CREATE TABLE ingestion_scopes (
    scope_id TEXT PRIMARY KEY,
    scope_kind TEXT NOT NULL,
    source_key TEXT NOT NULL,
    collector_kind TEXT NOT NULL
);
CREATE TABLE fact_work_items (
    work_item_id TEXT PRIMARY KEY,
    scope_id TEXT NOT NULL,
    generation_id TEXT NOT NULL,
    stage TEXT NOT NULL,
    domain TEXT NOT NULL,
    status TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    failure_class TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    visible_at TIMESTAMPTZ NULL
)`
	scratchPool, err := pgxpool.New(ctx, scratchDSN)
	if err != nil {
		t.Fatalf("open scratch pool: %v", err)
	}
	defer scratchPool.Close()
	if _, err := scratchPool.Exec(ctx, schema); err != nil {
		t.Fatalf("create dead-letter tables: %v", err)
	}

	seed := `
INSERT INTO ingestion_scopes (scope_id, scope_kind, source_key, collector_kind) VALUES
    ('s1', 'repository', 'repo-1', 'ck-a'),
    ('s2', 'repository', 'repo-2', 'ck-b');
INSERT INTO fact_work_items
    (work_item_id, scope_id, generation_id, stage, domain, status,
     attempt_count, failure_class, created_at, updated_at, visible_at)
VALUES
    ('w1', 's1', 'g1', 'build', 'd', 'dead_letter', 3, 'build_failed',
     '2026-03-04T05:06:07Z', '2026-03-04T08:06:07Z', '2026-03-04T07:06:07Z'),
    ('w2', 's2', 'g1', 'test', 'd', 'dead_letter', 1, NULL,
     '2026-03-04T05:06:07Z', '2026-03-04T06:06:07Z', NULL),
    ('w3', 's1', 'g1', 'build', 'd', 'pending', 0, NULL,
     '2026-03-04T05:06:07Z', '2026-03-04T09:06:07Z', NULL),
    ('w4', 's1', 'g1', 'build', 'd', 'dead_letter', 2, 'test_flake',
     '2026-03-04T05:06:07Z', '2026-03-04T04:06:07Z', NULL)`
	if _, err := scratchPool.Exec(ctx, seed); err != nil {
		t.Fatalf("seed dead-letter tables: %v", err)
	}

	store, err := NewStore(ctx, Config{DSN: scratchDSN, MaxConns: 2})
	if err != nil {
		t.Fatalf("open reader store: %v", err)
	}
	defer store.Close()

	items, err := store.ListDeadLetterWorkItems(ctx, apilibadmin.DeadLetterListFilter{})
	if err != nil {
		t.Fatalf("bounded list: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("bounded list rows = %d, want 3 (pending w3 excluded)", len(items))
	}
	if items[0].WorkItemID != "w1" || items[1].WorkItemID != "w2" || items[2].WorkItemID != "w4" {
		t.Fatalf("bounded list order = %v, want updated_at DESC (w1 w2 w4)", deadLetterRowIDs(items))
	}
	if items[0].CollectorKind != "ck-a" || items[1].CollectorKind != "ck-b" {
		t.Errorf("collector kinds = %q/%q, want the joined ck-a/ck-b", items[0].CollectorKind, items[1].CollectorKind)
	}
	if items[0].FailureClass == nil || *items[0].FailureClass != "build_failed" {
		t.Errorf("w1 failure_class = %v, want build_failed", items[0].FailureClass)
	}
	if items[1].FailureClass != nil || items[1].VisibleAt != nil {
		t.Errorf("w2 keeps nil pointers: %+v", items[1])
	}

	filtered, err := store.ListDeadLetterWorkItems(ctx, apilibadmin.DeadLetterListFilter{CollectorKind: "  ck-b  "})
	if err != nil {
		t.Fatalf("collector filter: %v", err)
	}
	if len(filtered) != 1 || filtered[0].WorkItemID != "w2" {
		t.Errorf("collector filter rows = %v, want [w2]", deadLetterRowIDs(filtered))
	}
	paged, err := store.ListDeadLetterWorkItems(ctx, apilibadmin.DeadLetterListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("limit 2: %v", err)
	}
	if len(paged) != 2 || paged[0].WorkItemID != "w1" {
		t.Errorf("limit 2 rows = %v, want [w1 w2]", deadLetterRowIDs(paged))
	}
	classed, err := store.ListDeadLetterWorkItems(ctx, apilibadmin.DeadLetterListFilter{FailureClass: "test_flake"})
	if err != nil {
		t.Fatalf("failure-class filter: %v", err)
	}
	if len(classed) != 1 || classed[0].WorkItemID != "w4" {
		t.Errorf("failure-class filter rows = %v, want [w4]", deadLetterRowIDs(classed))
	}

	// Windowed read: the core-exact updated_at range (>= after
	// inclusive, < before exclusive). w2 sits exactly on the after
	// bound (kept), w1 sits exactly on the before bound (dropped),
	// and w4 falls below the window (dropped).
	windowAfter := time.Date(2026, 3, 4, 6, 6, 7, 0, time.UTC)
	windowBefore := time.Date(2026, 3, 4, 8, 6, 7, 0, time.UTC)
	windowed, err := store.ListDeadLetterWorkItems(ctx, apilibadmin.DeadLetterListFilter{UpdatedAfter: &windowAfter, UpdatedBefore: &windowBefore})
	if err != nil {
		t.Fatalf("window filter: %v", err)
	}
	if len(windowed) != 1 || windowed[0].WorkItemID != "w2" {
		t.Errorf("window filter rows = %v, want [w2]", deadLetterRowIDs(windowed))
	}

	// A canceled context fails the read: the store honors the
	// caller's timeout bound instead of running unbounded.
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := store.ListDeadLetterWorkItems(canceled, apilibadmin.DeadLetterListFilter{}); err == nil {
		t.Error("canceled context: want the failure, got nil")
	}

	// Reader-role grant proof: a role holding only USAGE + SELECT on
	// both tables reads through the same join query text, and any
	// write refuses.
	role := fmt.Sprintf("adminread_deadletters_proof_reader_%d", os.Getpid())
	if _, err := scratchPool.Exec(ctx, "CREATE ROLE "+role+" WITH NOLOGIN"); err != nil {
		t.Fatalf("create proof role: %v", err)
	}
	defer func() {
		for _, cleanup := range []string{
			"REVOKE ALL ON fact_work_items FROM " + role,
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
		"GRANT SELECT ON fact_work_items TO " + role,
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
	query, args := buildListDeadLetterWorkItemsQuery(apilibadmin.DeadLetterListFilter{})
	pgRows, err := conn.Query(ctx, query, args...)
	if err != nil {
		t.Fatalf("reader-role list: %v", err)
	}
	roleItems, err := scanDeadLetters(pgRows)
	pgRows.Close()
	if err != nil {
		t.Fatalf("reader-role scan: %v", err)
	}
	if len(roleItems) != 3 {
		t.Errorf("reader-role rows = %d, want 3 (SELECT-only on both tables suffices)", len(roleItems))
	}
	// The windowed SELECT runs under the same SELECT-only grants:
	// the reader role reads the bounded window (at-after w2 kept,
	// at-before w1 dropped) with no extra privilege.
	roleWindowAfter := time.Date(2026, 3, 4, 6, 6, 7, 0, time.UTC)
	roleWindowBefore := time.Date(2026, 3, 4, 8, 6, 7, 0, time.UTC)
	roleQuery, roleArgs := buildListDeadLetterWorkItemsQuery(apilibadmin.DeadLetterListFilter{UpdatedAfter: &roleWindowAfter, UpdatedBefore: &roleWindowBefore})
	roleWindowRows, err := conn.Query(ctx, roleQuery, roleArgs...)
	if err != nil {
		t.Fatalf("reader-role window list: %v", err)
	}
	roleWindowItems, err := scanDeadLetters(roleWindowRows)
	roleWindowRows.Close()
	if err != nil {
		t.Fatalf("reader-role window scan: %v", err)
	}
	if len(roleWindowItems) != 1 || roleWindowItems[0].WorkItemID != "w2" {
		t.Errorf("reader-role window rows = %v, want [w2]", deadLetterRowIDs(roleWindowItems))
	}
	if _, err := conn.Exec(ctx, "INSERT INTO fact_work_items (work_item_id) VALUES ('wx')"); err == nil {
		t.Error("reader-role write succeeded, want a permission refusal")
	}
}

// deadLetterRowIDs renders row identities for failure output.
func deadLetterRowIDs(items []apilibadmin.DeadLetterWorkItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.WorkItemID)
	}
	return ids
}

// withDatabase repoints a URL DSN at a scratch database through the
// path segment. (pgx ConnConfig.ConnString replays the original DSN
// text and silently drops a mutated Database field, so the override
// must happen on the URL itself.) Non-URL DSNs fail the test instead
// of connecting somewhere unintended.
func withDatabase(t *testing.T, dsn, database string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatalf("fixture DSN is not a URL DSN, refusing to guess the scratch target: %v", err)
	}
	u.Path = "/" + database
	return u.String()
}

// rowIDs renders row identities for failure output.
func rowIDs(items []apilibadmin.WorkItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.WorkItemID)
	}
	return ids
}
