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

// TestAdminReadDecisionsLiveProof is the end-to-end pool/reader proof
// for the decisions lane: it builds a scratch database carrying the
// core projection-decision tables with the real core column types,
// opens the reader store against it, and proves the bounded list
// (ordering, type gate, default limit), the per-decision evidence
// fan-out (ordering, nullable fact_id, detail fold), plus the
// reader-role grant (SELECT-only role reads; writes refuse). It runs
// only when the fixture environment sets ESHU_TEST_PG_DSN; otherwise
// it skips, per the sessionstore fixture convention.
func TestAdminReadDecisionsLiveProof(t *testing.T) {
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

	scratch := fmt.Sprintf("adminread_decisions_proof_%d_%d", os.Getpid(), time.Now().UTC().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+scratch); err != nil {
		t.Fatalf("create scratch database: %v", err)
	}
	defer func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+scratch+" WITH (FORCE)")
	}()

	scratchDSN := withDatabase(t, base, scratch)
	schema := `
CREATE TABLE projection_decisions (
    decision_id TEXT PRIMARY KEY,
    decision_type TEXT NOT NULL,
    repository_id TEXT NOT NULL,
    source_run_id TEXT NOT NULL,
    work_item_id TEXT NOT NULL,
    subject TEXT NOT NULL,
    confidence_score DOUBLE PRECISION NOT NULL,
    confidence_reason TEXT NOT NULL,
    provenance_summary JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE projection_decision_evidence (
    evidence_id TEXT PRIMARY KEY,
    decision_id TEXT NOT NULL,
    fact_id TEXT NULL,
    evidence_kind TEXT NOT NULL,
    detail JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
)`
	scratchPool, err := pgxpool.New(ctx, scratchDSN)
	if err != nil {
		t.Fatalf("open scratch pool: %v", err)
	}
	defer scratchPool.Close()
	if _, err := scratchPool.Exec(ctx, schema); err != nil {
		t.Fatalf("create decision tables: %v", err)
	}

	seed := `
INSERT INTO projection_decisions
    (decision_id, decision_type, repository_id, source_run_id,
     work_item_id, subject, confidence_score, confidence_reason,
     provenance_summary, created_at)
VALUES
    ('d1', 'merge', 'r1', 's1', 'w1', 'subject-a', 0.9, 'reason-a',
     '{"planner":"p"}', '2026-03-04T05:06:07Z'),
    ('d2', 'split', 'r1', 's1', 'w2', 'subject-b', 0.5, 'reason-b',
     '{}', '2026-03-04T06:06:07Z'),
    ('d3', 'merge', 'r1', 's2', 'w3', 'subject-c', 0.7, 'reason-c',
     '{}', '2026-03-04T04:06:07Z');
INSERT INTO projection_decision_evidence
    (evidence_id, decision_id, fact_id, evidence_kind, detail, created_at)
VALUES
    ('e1', 'd1', 'f1', 'file', '{"p":"x"}', '2026-03-04T05:06:08Z'),
    ('e2', 'd1', NULL, 'span', '{}', '2026-03-04T05:06:09Z')`
	if _, err := scratchPool.Exec(ctx, seed); err != nil {
		t.Fatalf("seed decision tables: %v", err)
	}

	store, err := NewStore(ctx, Config{DSN: scratchDSN, MaxConns: 2})
	if err != nil {
		t.Fatalf("open reader store: %v", err)
	}
	defer store.Close()

	decisions, err := store.ListDecisions(ctx, apilibadmin.DecisionQueryFilter{RepositoryID: "r1", SourceRunID: "s1"})
	if err != nil {
		t.Fatalf("bounded list: %v", err)
	}
	if len(decisions) != 2 {
		t.Fatalf("bounded list rows = %d, want 2 (d3 rides another run)", len(decisions))
	}
	if decisions[0].DecisionID != "d1" || decisions[1].DecisionID != "d2" {
		t.Fatalf("bounded list order = %v, want created_at ASC (d1 d2)", decisionRowIDs(decisions))
	}
	if decisions[0].ProvenanceSummary["planner"] != "p" {
		t.Errorf("d1 provenance = %v, want planner p", decisions[0].ProvenanceSummary)
	}
	if decisions[0].ConfidenceScore != 0.9 {
		t.Errorf("d1 confidence = %v, want 0.9", decisions[0].ConfidenceScore)
	}

	merge := "merge"
	typed, err := store.ListDecisions(ctx, apilibadmin.DecisionQueryFilter{RepositoryID: "r1", SourceRunID: "s1", DecisionType: &merge})
	if err != nil {
		t.Fatalf("type gate: %v", err)
	}
	if len(typed) != 1 || typed[0].DecisionID != "d1" {
		t.Errorf("type gate rows = %v, want [d1]", decisionRowIDs(typed))
	}
	paged, err := store.ListDecisions(ctx, apilibadmin.DecisionQueryFilter{RepositoryID: "r1", SourceRunID: "s1", Limit: 1})
	if err != nil {
		t.Fatalf("limit 1: %v", err)
	}
	if len(paged) != 1 || paged[0].DecisionID != "d1" {
		t.Errorf("limit 1 rows = %v, want [d1]", decisionRowIDs(paged))
	}

	evidence, err := store.ListEvidence(ctx, "d1")
	if err != nil {
		t.Fatalf("evidence fan-out: %v", err)
	}
	if len(evidence) != 2 || evidence[0].EvidenceID != "e1" || evidence[1].EvidenceID != "e2" {
		t.Fatalf("evidence rows = %v, want created_at ASC (e1 e2)", evidenceRowIDs(evidence))
	}
	if evidence[0].FactID == nil || *evidence[0].FactID != "f1" {
		t.Errorf("e1 fact_id = %v, want f1", evidence[0].FactID)
	}
	if evidence[0].Detail["p"] != "x" {
		t.Errorf("e1 detail = %v, want p:x", evidence[0].Detail)
	}
	if evidence[1].FactID != nil {
		t.Errorf("e2 keeps a fact_id: %+v", evidence[1])
	}
	unknown, err := store.ListEvidence(ctx, "no-such-decision")
	if err != nil {
		t.Fatalf("unknown decision evidence: %v", err)
	}
	if len(unknown) != 0 {
		t.Errorf("unknown decision evidence rows = %d, want 0 (never an error)", len(unknown))
	}

	// A canceled context fails the read: the store honors the
	// caller's bound instead of running unbounded.
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := store.ListDecisions(canceled, apilibadmin.DecisionQueryFilter{}); err == nil {
		t.Error("canceled context: want the failure, got nil")
	}

	// Reader-role grant proof: a role holding only USAGE + SELECT on
	// both tables reads through the same query texts, and any write
	// refuses.
	role := fmt.Sprintf("adminread_decisions_proof_reader_%d", os.Getpid())
	if _, err := scratchPool.Exec(ctx, "CREATE ROLE "+role+" WITH NOLOGIN"); err != nil {
		t.Fatalf("create proof role: %v", err)
	}
	defer func() {
		for _, cleanup := range []string{
			"REVOKE ALL ON projection_decisions FROM " + role,
			"REVOKE ALL ON projection_decision_evidence FROM " + role,
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
		"GRANT SELECT ON projection_decisions TO " + role,
		"GRANT SELECT ON projection_decision_evidence TO " + role,
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
	query, args := buildListDecisionsQuery(apilibadmin.DecisionQueryFilter{RepositoryID: "r1", SourceRunID: "s1"})
	pgRows, err := conn.Query(ctx, query, args...)
	if err != nil {
		t.Fatalf("reader-role list: %v", err)
	}
	roleDecisions, err := scanDecisions(pgRows)
	pgRows.Close()
	if err != nil {
		t.Fatalf("reader-role scan: %v", err)
	}
	if len(roleDecisions) != 2 {
		t.Errorf("reader-role rows = %d, want 2", len(roleDecisions))
	}
	evQuery, evArgs := buildListEvidenceQuery("d1")
	evPgRows, err := conn.Query(ctx, evQuery, evArgs...)
	if err != nil {
		t.Fatalf("reader-role evidence: %v", err)
	}
	roleEvidence, err := scanEvidence(evPgRows)
	evPgRows.Close()
	if err != nil {
		t.Fatalf("reader-role evidence scan: %v", err)
	}
	if len(roleEvidence) != 2 {
		t.Errorf("reader-role evidence rows = %d, want 2", len(roleEvidence))
	}
	if _, err := conn.Exec(ctx, "SET ROLE NONE"); err != nil {
		t.Fatalf("drop proof role: %v", err)
	}
	if _, err := conn.Exec(ctx, "SET ROLE "+role); err != nil {
		t.Fatalf("re-assume proof role: %v", err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO projection_decisions (decision_id) VALUES ('w-nope')"); err == nil {
		t.Error("reader-role write: want the refusal, got nil")
	}
	if _, err := conn.Exec(ctx, "SET ROLE NONE"); err != nil {
		t.Fatalf("release proof role: %v", err)
	}
}

func decisionRowIDs(items []apilibadmin.DecisionRow) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.DecisionID)
	}
	return ids
}

func evidenceRowIDs(items []apilibadmin.EvidenceRow) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.EvidenceID)
	}
	return ids
}
