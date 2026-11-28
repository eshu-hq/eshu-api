// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package adminread

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// TestBuildListDecisionsQueryPinsCoreColumns pins the reader SELECT
// against the core decisions read: the same columns in the same
// order, the same repository/run predicate, the same decision_type
// gate, the same default limit, and the core-exact created_at ASC +
// decision_id ASC page.
func TestBuildListDecisionsQueryPinsCoreColumns(t *testing.T) {
	t.Parallel()

	decisionType := " merge "
	query, args := buildListDecisionsQuery(apilibadmin.DecisionQueryFilter{
		RepositoryID: "r1",
		SourceRunID:  "s1",
		DecisionType: &decisionType,
		Limit:        7,
	})

	for _, column := range []string{
		"decision_id", "decision_type", "repository_id", "source_run_id",
		"work_item_id", "subject", "confidence_score", "confidence_reason",
		"provenance_summary", "created_at",
	} {
		if !strings.Contains(query, column) {
			t.Errorf("query lacks column %s:\n%s", column, query)
		}
	}
	for _, fragment := range []string{
		"FROM projection_decisions",
		"WHERE repository_id = $1",
		"AND source_run_id = $2",
		"AND ($3 = '' OR decision_type = $3)",
		"ORDER BY created_at ASC, decision_id ASC LIMIT $4",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query lacks fragment %q:\n%s", fragment, query)
		}
	}
	if len(args) != 4 {
		t.Fatalf("args = %v, want 4 bound values", args)
	}
	if args[0] != "r1" || args[1] != "s1" || args[2] != " merge " || args[3] != 7 {
		t.Errorf("args = %v, want the pair plus the raw type plus limit 7", args)
	}
}

// TestBuildListDecisionsQueryDefaults pins the unset legs: a nil
// decision type binds the inert empty gate (reads unfiltered, like
// core) and an absent limit defaults to 100.
func TestBuildListDecisionsQueryDefaults(t *testing.T) {
	t.Parallel()

	query, args := buildListDecisionsQuery(apilibadmin.DecisionQueryFilter{
		RepositoryID: "r1",
		SourceRunID:  "s1",
	})
	if !strings.Contains(query, "AND ($3 = '' OR decision_type = $3)") {
		t.Errorf("bare query lacks the inert type gate:\n%s", query)
	}
	if len(args) != 4 {
		t.Fatalf("args = %v, want the pair plus the empty gate plus the default limit", args)
	}
	if args[2] != "" || args[3] != 100 {
		t.Errorf("args = %v, want [r1 s1 '' 100]", args)
	}
}

// TestBuildListEvidenceQueryPinsCoreShape pins the evidence fan-out
// SELECT: the core column set in core order over
// projection_decision_evidence, one decision_id bind, and the
// core-exact created_at ASC + evidence_id ASC page.
func TestBuildListEvidenceQueryPinsCoreShape(t *testing.T) {
	t.Parallel()

	query, args := buildListEvidenceQuery("d1")
	for _, column := range []string{
		"evidence_id", "decision_id", "fact_id", "evidence_kind", "detail", "created_at",
	} {
		if !strings.Contains(query, column) {
			t.Errorf("query lacks column %s:\n%s", column, query)
		}
	}
	for _, fragment := range []string{
		"FROM projection_decision_evidence",
		"WHERE decision_id = $1",
		"ORDER BY created_at ASC, evidence_id ASC",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query lacks fragment %q:\n%s", fragment, query)
		}
	}
	if len(args) != 1 || args[0] != "d1" {
		t.Errorf("args = %v, want [d1]", args)
	}
}

// stubDecisionRows drives scanDecisions without a live database: one
// row per programmed record, scanned in the SELECT column order.
type stubDecisionRows struct {
	records [][]any
	pos     int
	err     error
	closed  bool
}

func (r *stubDecisionRows) Next() bool { return r.pos < len(r.records) }

func (r *stubDecisionRows) Scan(dest ...any) error {
	if r.pos >= len(r.records) {
		return context.Canceled
	}
	record := r.records[r.pos]
	r.pos++
	if len(dest) != len(record) {
		return context.Canceled
	}
	for i, value := range record {
		switch slot := dest[i].(type) {
		case *string:
			*slot = value.(string)
		case *float64:
			*slot = value.(float64)
		case *time.Time:
			*slot = value.(time.Time)
		case *[]byte:
			*slot = value.([]byte)
		default:
			return context.Canceled
		}
	}
	return nil
}

func (r *stubDecisionRows) Err() error { return r.err }

func (r *stubDecisionRows) Close() { r.closed = true }

// The pgx.Rows remainder: the scan only drives Next/Scan/Err/Close,
// so the rest stay inert.
func (r *stubDecisionRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *stubDecisionRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *stubDecisionRows) Values() ([]any, error) { return nil, nil }

func (r *stubDecisionRows) RawValues() [][]byte { return nil }

func (r *stubDecisionRows) Conn() *pgx.Conn { return nil }

// stubEvidenceRows drives scanEvidence without a live database: one
// row per programmed record, scanned in the SELECT column order.
type stubEvidenceRows struct {
	records [][]any
	pos     int
	err     error
	closed  bool
}

func (r *stubEvidenceRows) Next() bool { return r.pos < len(r.records) }

func (r *stubEvidenceRows) Scan(dest ...any) error {
	if r.pos >= len(r.records) {
		return context.Canceled
	}
	record := r.records[r.pos]
	r.pos++
	if len(dest) != len(record) {
		return context.Canceled
	}
	for i, value := range record {
		switch slot := dest[i].(type) {
		case *string:
			*slot = value.(string)
		case *sql.NullString:
			*slot = value.(sql.NullString)
		case *time.Time:
			*slot = value.(time.Time)
		case *[]byte:
			*slot = value.([]byte)
		default:
			return context.Canceled
		}
	}
	return nil
}

func (r *stubEvidenceRows) Err() error { return r.err }

func (r *stubEvidenceRows) Close() { r.closed = true }

// The pgx.Rows remainder: the scan only drives Next/Scan/Err/Close,
// so the rest stay inert.
func (r *stubEvidenceRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *stubEvidenceRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *stubEvidenceRows) Values() ([]any, error) { return nil, nil }

func (r *stubEvidenceRows) RawValues() [][]byte { return nil }

func (r *stubEvidenceRows) Conn() *pgx.Conn { return nil }

// TestScanDecisionsFoldsJSONB pins the scan against the core fold:
// provenance_summary unfolds from JSONB, empty JSONB leaves a nil
// map, and a malformed document surfaces instead of silent partial
// rows.
func TestScanDecisionsFoldsJSONB(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	rows := &stubDecisionRows{records: [][]any{
		{
			"d1", "merge", "r1", "s1", "w1", "subject-a", 0.9, "reason-a",
			[]byte(`{"planner":"p"}`),
			created,
		},
		{
			"d2", "split", "r1", "s1", "w2", "subject-b", 0.5, "reason-b",
			[]byte{},
			created,
		},
	}}

	items, err := scanDecisions(rows)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	rows.Close()
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	first := items[0]
	if first.DecisionID != "d1" || first.DecisionType != "merge" || first.RepositoryID != "r1" || first.SourceRunID != "s1" {
		t.Errorf("first row identity = %+v, want d1/merge/r1/s1", first)
	}
	if first.ConfidenceScore != 0.9 || first.ProvenanceSummary["planner"] != "p" {
		t.Errorf("first row fold = %+v, want score 0.9 with planner p", first)
	}
	if items[1].ProvenanceSummary != nil {
		t.Errorf("second row keeps a provenance map: %+v", items[1])
	}

	ragged := &stubDecisionRows{records: [][]any{{"only-one-column"}}}
	if _, err := scanDecisions(ragged); err == nil {
		t.Error("ragged row: want a scan failure, got nil")
	}
	malformed := &stubDecisionRows{records: [][]any{
		{"d9", "merge", "r1", "s1", "w9", "s", 0.1, "r", []byte(`{"planner":`), created},
	}}
	if _, err := scanDecisions(malformed); err == nil {
		t.Error("malformed provenance: want the failure, got nil")
	}
	failing := &stubDecisionRows{err: context.DeadlineExceeded}
	if _, err := scanDecisions(failing); err == nil {
		t.Error("cursor error: want the failure, got nil")
	}
}

// TestScanEvidenceFoldsNullables pins the evidence scan: fact_id
// stays nil when NULL and present values map through, detail unfolds
// from JSONB, and cursor failures surface.
func TestScanEvidenceFoldsNullables(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	fact := "f1"
	rows := &stubEvidenceRows{records: [][]any{
		{
			"e1", "d1",
			sql.NullString{String: fact, Valid: true},
			"file", []byte(`{"p":"x"}`), created,
		},
		{
			"e2", "d1",
			sql.NullString{Valid: false},
			"span",
			[]byte{},
			created,
		},
	}}

	items, err := scanEvidence(rows)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	rows.Close()
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].FactID == nil || *items[0].FactID != fact {
		t.Errorf("first fact_id = %v, want f1", items[0].FactID)
	}
	if items[0].Detail["p"] != "x" {
		t.Errorf("first detail = %v, want p:x", items[0].Detail)
	}
	if items[1].FactID != nil || items[1].Detail != nil {
		t.Errorf("second row keeps pointers: %+v", items[1])
	}

	ragged := &stubEvidenceRows{records: [][]any{{"only-one-column"}}}
	if _, err := scanEvidence(ragged); err == nil {
		t.Error("ragged row: want a scan failure, got nil")
	}
	failing := &stubEvidenceRows{err: context.DeadlineExceeded}
	if _, err := scanEvidence(failing); err == nil {
		t.Error("cursor error: want the failure, got nil")
	}
}

// TestScanEvidenceSurfacesFailures pins the evidence scan's error
// legs: malformed detail JSON and a trailing cursor error surface
// wrapped errors (never partial rows or panics).
func TestScanEvidenceSurfacesFailures(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	badJSON := &stubEvidenceRows{records: [][]any{
		{"e1", "d1", sql.NullString{Valid: false}, "file", []byte(`{"p":`), created},
	}}
	if _, err := scanEvidence(badJSON); err == nil {
		t.Error("malformed detail JSON: want wrapped error, got nil")
	}
	trailing := &stubEvidenceRows{
		records: [][]any{
			{"e1", "d1", sql.NullString{Valid: false}, "file", []byte(`{}`), created},
		},
		err: context.DeadlineExceeded,
	}
	if _, err := scanEvidence(trailing); err == nil {
		t.Error("trailing cursor error: want wrapped error, got nil")
	}
}

// TestListEvidenceThroughPool pins the evidence fan-out read path:
// success folds rows, and a nil store fails closed without a query.
func TestListEvidenceThroughPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	created := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	rows := &stubEvidenceRows{records: [][]any{
		{"e1", "d1", sql.NullString{String: "f1", Valid: true}, "file", []byte(`{}`), created},
	}}
	store := newStore(&stubDecisionPool{rows: rows})
	items, err := store.ListEvidence(ctx, "d1")
	if err != nil {
		t.Fatalf("list evidence: %v", err)
	}
	if len(items) != 1 || items[0].EvidenceID != "e1" {
		t.Fatalf("items = %+v, want the one row", items)
	}
	if !rows.closed {
		t.Error("cursor left open after the read")
	}

	var nilStore *Store
	if _, err := nilStore.ListEvidence(ctx, "d1"); err == nil {
		t.Error("nil store evidence: want a not-configured error, got nil")
	}
}

// stubDecisionPool binds canned decisions rows or a canned query
// error behind the adminread Pool seam: ListDecisions success and
// query-failure legs without a live database.
type stubDecisionPool struct {
	rows   pgx.Rows
	err    error
	closed bool
}

func (p *stubDecisionPool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.rows, nil
}

func (p *stubDecisionPool) Close() { p.closed = true }

// TestListDecisionsThroughPool pins the store read path: success
// folds rows and releases the cursor, a query failure surfaces, and
// a nil or pool-less store fails closed.
func TestListDecisionsThroughPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	created := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	rows := &stubDecisionRows{records: [][]any{
		{
			"d1", "merge", "r1", "s1", "w1", "subject-a", 0.9, "reason-a",
			[]byte(`{"planner":"p"}`),
			created,
		},
	}}
	store := newStore(&stubDecisionPool{rows: rows})
	items, err := store.ListDecisions(ctx, apilibadmin.DecisionQueryFilter{RepositoryID: "r1", SourceRunID: "s1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].DecisionID != "d1" {
		t.Fatalf("items = %+v, want the one row", items)
	}
	if !rows.closed {
		t.Error("cursor left open after the read")
	}

	failing := newStore(&stubDecisionPool{err: context.DeadlineExceeded})
	if _, err := failing.ListDecisions(ctx, apilibadmin.DecisionQueryFilter{}); err == nil {
		t.Error("query failure: want the error, got nil")
	}
	if _, err := failing.ListEvidence(ctx, "d1"); err == nil {
		t.Error("evidence query failure: want the error, got nil")
	}

	var nilStore *Store
	if _, err := nilStore.ListDecisions(ctx, apilibadmin.DecisionQueryFilter{}); err == nil {
		t.Error("nil store: want a not-configured error, got nil")
	}
	if _, err := nilStore.ListEvidence(ctx, "d1"); err == nil {
		t.Error("nil store evidence: want a not-configured error, got nil")
	}
	if _, err := (&Store{}).ListDecisions(ctx, apilibadmin.DecisionQueryFilter{}); err == nil {
		t.Error("closed store: want a not-configured error, got nil")
	}
	if _, err := (&Store{}).ListEvidence(ctx, "d1"); err == nil {
		t.Error("closed store evidence: want a not-configured error, got nil")
	}
}
