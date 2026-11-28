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

// TestAdminReadInputInvalidFactsLiveProof is the end-to-end
// pool/reader proof for the input-invalid-facts lane: it builds a
// scratch database carrying the core reducer_input_invalid_facts
// table with the real core column types, opens the reader store
// against it, and proves the bounded list (ordering, selectors,
// default limit) plus the reader-role grant (SELECT-only role reads;
// writes refuse). It runs only when the fixture environment sets
// ESHU_TEST_PG_DSN; otherwise it skips, per the sessionstore fixture
// convention.
func TestAdminReadInputInvalidFactsLiveProof(t *testing.T) {
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

	scratch := fmt.Sprintf("adminread_inputinvalid_proof_%d_%d", os.Getpid(), time.Now().UTC().UnixNano())
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
    source_key TEXT NOT NULL
);
CREATE TABLE reducer_input_invalid_facts (
    fact_id TEXT NOT NULL,
    fact_kind TEXT NOT NULL,
    missing_field TEXT NOT NULL,
    failure_class TEXT NOT NULL,
    domain TEXT NOT NULL,
    scope_id TEXT NOT NULL REFERENCES ingestion_scopes(scope_id) ON DELETE CASCADE,
    generation_id TEXT NOT NULL,
    decided_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (scope_id, generation_id, fact_id, missing_field, domain)
)`
	scratchPool, err := pgxpool.New(ctx, scratchDSN)
	if err != nil {
		t.Fatalf("open scratch pool: %v", err)
	}
	defer scratchPool.Close()
	if _, err := scratchPool.Exec(ctx, schema); err != nil {
		t.Fatalf("create quarantine tables: %v", err)
	}

	seed := `
INSERT INTO ingestion_scopes (scope_id, scope_kind, source_key)
VALUES ('scope-1', 'repository', 'repo-1');
INSERT INTO reducer_input_invalid_facts
    (fact_id, fact_kind, missing_field, failure_class, domain,
     scope_id, generation_id, decided_at)
VALUES
    ('fact-1', 'typed-payload', 'spec_replicas', 'input_invalid', 'deploy',
     'scope-1', 'gen-1', '2026-03-04T05:06:07Z'),
    ('fact-2', 'typed-payload', 'runs', 'input_invalid', 'ci',
     'scope-1', 'gen-1', '2026-03-04T06:06:07Z'),
    ('fact-3', 'typed-payload', 'spec_replicas', 'input_invalid', 'deploy',
     'scope-1', 'gen-2', '2026-03-04T04:06:07Z')`
	if _, err := scratchPool.Exec(ctx, seed); err != nil {
		t.Fatalf("seed quarantine tables: %v", err)
	}

	store, err := NewStore(ctx, Config{DSN: scratchDSN, MaxConns: 2})
	if err != nil {
		t.Fatalf("open reader store: %v", err)
	}
	defer store.Close()

	items, err := store.ListReducerInputInvalidFacts(ctx, apilibadmin.InputInvalidFactListFilter{
		ScopeID:      "scope-1",
		GenerationID: "gen-1",
	})
	if err != nil {
		t.Fatalf("bounded list: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("bounded list rows = %d, want 2 (fact-3 rides another generation)", len(items))
	}
	if items[0].FactID != "fact-2" || items[1].FactID != "fact-1" {
		t.Fatalf("bounded list order = %v, want decided_at DESC (fact-2 fact-1)", inputInvalidFactIDs(items))
	}
	if items[0].MissingField != "runs" || items[0].FailureClass != "input_invalid" {
		t.Errorf("fact-2 classification = %+v, want the reducer verdict kept", items[0])
	}

	filtered, err := store.ListReducerInputInvalidFacts(ctx, apilibadmin.InputInvalidFactListFilter{
		ScopeID:      "scope-1",
		GenerationID: "gen-1",
		Domain:       "ci",
	})
	if err != nil {
		t.Fatalf("domain filter: %v", err)
	}
	if len(filtered) != 1 || filtered[0].FactID != "fact-2" {
		t.Errorf("domain rows = %v, want [fact-2]", inputInvalidFactIDs(filtered))
	}
	paged, err := store.ListReducerInputInvalidFacts(ctx, apilibadmin.InputInvalidFactListFilter{
		ScopeID:      "scope-1",
		GenerationID: "gen-1",
		Limit:        1,
	})
	if err != nil {
		t.Fatalf("limit 1: %v", err)
	}
	if len(paged) != 1 || paged[0].FactID != "fact-2" {
		t.Errorf("limit 1 rows = %v, want [fact-2]", inputInvalidFactIDs(paged))
	}

	// A canceled context fails the read: the store honors the
	// caller's bound instead of running unbounded.
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := store.ListReducerInputInvalidFacts(canceled, apilibadmin.InputInvalidFactListFilter{}); err == nil {
		t.Error("canceled context: want the failure, got nil")
	}

	// Reader-role grant proof: a role holding only USAGE + SELECT on
	// both tables reads through the same query text, and any write
	// refuses.
	role := fmt.Sprintf("adminread_inputinvalid_proof_reader_%d", os.Getpid())
	if _, err := scratchPool.Exec(ctx, "CREATE ROLE "+role+" WITH NOLOGIN"); err != nil {
		t.Fatalf("create proof role: %v", err)
	}
	defer func() {
		for _, cleanup := range []string{
			"REVOKE ALL ON reducer_input_invalid_facts FROM " + role,
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
		"GRANT SELECT ON reducer_input_invalid_facts TO " + role,
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
	query, args := buildListReducerInputInvalidFactsQuery(apilibadmin.InputInvalidFactListFilter{
		ScopeID:      "scope-1",
		GenerationID: "gen-1",
	})
	pgRows, err := conn.Query(ctx, query, args...)
	if err != nil {
		t.Fatalf("reader-role list: %v", err)
	}
	roleItems, err := scanInputInvalidFacts(pgRows)
	pgRows.Close()
	if err != nil {
		t.Fatalf("reader-role scan: %v", err)
	}
	if len(roleItems) != 2 {
		t.Errorf("reader-role rows = %d, want 2", len(roleItems))
	}
	if _, err := conn.Exec(ctx, "SET ROLE NONE"); err != nil {
		t.Fatalf("drop proof role: %v", err)
	}
	if _, err := conn.Exec(ctx, "SET ROLE "+role); err != nil {
		t.Fatalf("re-assume proof role: %v", err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO reducer_input_invalid_facts (fact_id) VALUES ('w-nope')"); err == nil {
		t.Error("reader-role write: want the refusal, got nil")
	}
	if _, err := conn.Exec(ctx, "SET ROLE NONE"); err != nil {
		t.Fatalf("release proof role: %v", err)
	}
}

func inputInvalidFactIDs(items []apilibadmin.InputInvalidFact) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.FactID)
	}
	return ids
}
