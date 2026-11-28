// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package adminread

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// TestBuildListReducerInputInvalidFactsQueryPinsCoreColumns pins the
// reader SELECT against the core quarantine read: the same columns
// in the same order, the same scope/generation predicate, the same
// domain and fact_kind filters, the same scope-authorizing access
// clause, the same default limit, and the core-exact decided_at DESC
// + fact_id ASC + missing_field ASC page.
func TestBuildListReducerInputInvalidFactsQueryPinsCoreColumns(t *testing.T) {
	t.Parallel()

	query, args := buildListReducerInputInvalidFactsQuery(apilibadmin.InputInvalidFactListFilter{
		ScopeID:      "scope-1",
		GenerationID: "gen-1",
		Domain:       " deploy ",
		FactKind:     " typed-payload ",
		Limit:        7,
	})

	for _, column := range []string{
		"fact_id", "fact_kind", "missing_field", "failure_class",
		"domain", "scope_id", "generation_id", "decided_at",
	} {
		if !strings.Contains(query, column) {
			t.Errorf("query lacks column %s:\n%s", column, query)
		}
	}
	for _, fragment := range []string{
		"FROM reducer_input_invalid_facts AS quarantine",
		"JOIN ingestion_scopes AS scope ON scope.scope_id = quarantine.scope_id",
		"WHERE quarantine.scope_id = $1",
		"AND quarantine.generation_id = $2",
		"AND quarantine.domain = $3",
		"AND quarantine.fact_kind = $4",
		"ORDER BY quarantine.decided_at DESC, quarantine.fact_id ASC, quarantine.missing_field ASC LIMIT $5",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query lacks fragment %q:\n%s", fragment, query)
		}
	}
	if len(args) != 5 {
		t.Fatalf("args = %v, want 5 bound values", args)
	}
	if args[0] != "scope-1" || args[1] != "gen-1" || args[2] != "deploy" || args[3] != "typed-payload" || args[4] != 7 {
		t.Errorf("args = %v, want the pair plus the trimmed selectors plus limit 7", args)
	}
}

// TestBuildListReducerInputInvalidFactsQueryDefaults pins the unset
// legs: blank selectors bind no predicate (the builder trims, like
// core, so blanks stay absent), an absent limit defaults to 100, and
// empty grants read unscoped (no access clause), like core.
func TestBuildListReducerInputInvalidFactsQueryDefaults(t *testing.T) {
	t.Parallel()

	query, args := buildListReducerInputInvalidFactsQuery(apilibadmin.InputInvalidFactListFilter{
		ScopeID:      "scope-1",
		GenerationID: "gen-1",
	})
	if strings.Contains(query, "AND quarantine.domain") || strings.Contains(query, "AND quarantine.fact_kind") {
		t.Errorf("bare query binds a selector predicate:\n%s", query)
	}
	if strings.Contains(query, "scope_kind") {
		t.Errorf("bare query binds the access clause:\n%s", query)
	}
	if !strings.Contains(query, "LIMIT $3") {
		t.Errorf("bare query lacks the default-limit bind:\n%s", query)
	}
	if len(args) != 3 {
		t.Fatalf("args = %v, want 3 bound values", args)
	}
	if args[2] != 100 {
		t.Errorf("args = %v, want [scope-1 gen-1 100]", args)
	}
}

// TestBuildListReducerInputInvalidFactsQueryGrants pins the scoped
// access clause: granted repositories and scopes bind through the
// ingestion_scopes join, mirroring the core authorization.
func TestBuildListReducerInputInvalidFactsQueryGrants(t *testing.T) {
	t.Parallel()

	query, args := buildListReducerInputInvalidFactsQuery(apilibadmin.InputInvalidFactListFilter{
		ScopeID:              "scope-1",
		GenerationID:         "gen-1",
		AllowedRepositoryIDs: []string{"repo-1"},
		AllowedScopeIDs:      []string{"scope-9"},
		Limit:                10,
	})
	if !strings.Contains(query, "((scope.scope_kind = 'repository' AND scope.source_key = ANY($3)) OR quarantine.scope_id = ANY($4))") {
		t.Errorf("query lacks the scoped access clause:\n%s", query)
	}
	if len(args) != 5 {
		t.Fatalf("args = %v, want 5 bound values", args)
	}
	if args[4] != 10 {
		t.Errorf("args = %v, want the caller limit last", args)
	}
}

// stubInputInvalidRows drives scanInputInvalidFacts without a live
// database: one row per programmed record, scanned in the SELECT
// column order.
type stubInputInvalidRows struct {
	records [][]any
	pos     int
	err     error
	closed  bool
}

func (r *stubInputInvalidRows) Next() bool { return r.pos < len(r.records) }

func (r *stubInputInvalidRows) Scan(dest ...any) error {
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
		case *time.Time:
			*slot = value.(time.Time)
		default:
			return context.Canceled
		}
	}
	return nil
}

func (r *stubInputInvalidRows) Err() error { return r.err }

func (r *stubInputInvalidRows) Close() { r.closed = true }

// The pgx.Rows remainder: the scan only drives Next/Scan/Err/Close,
// so the rest stay inert.
func (r *stubInputInvalidRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *stubInputInvalidRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *stubInputInvalidRows) Values() ([]any, error) { return nil, nil }

func (r *stubInputInvalidRows) RawValues() [][]byte { return nil }

func (r *stubInputInvalidRows) Conn() *pgx.Conn { return nil }

// TestScanInputInvalidFactsKeepsClassification pins the scan against
// the core fold: every column maps verbatim (the quarantine keeps
// the reducer's classification — missing_field and failure_class are
// never zero-valued or re-derived), decided_at maps to the instant,
// and cursor failures surface wrapped errors (never partial rows or
// panics).
func TestScanInputInvalidFactsKeepsClassification(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	rows := &stubInputInvalidRows{records: [][]any{
		{"fact-1", "typed-payload", "spec_replicas", "input_invalid", "deploy", "scope-1", "gen-1", created},
		{"fact-2", "typed-payload", "runs", "input_invalid", "ci", "scope-1", "gen-1", created},
	}}

	items, err := scanInputInvalidFacts(rows)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	rows.Close()
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	first := items[0]
	if first.FactID != "fact-1" || first.FactKind != "typed-payload" || first.MissingField != "spec_replicas" {
		t.Errorf("first row identity = %+v, want fact-1/typed-payload/spec_replicas", first)
	}
	if first.FailureClass != "input_invalid" || first.Domain != "deploy" {
		t.Errorf("first row classification = %+v, want the reducer verdict kept", first)
	}
	if first.ScopeID != "scope-1" || first.GenerationID != "gen-1" {
		t.Errorf("first row scope = %+v, want scope-1/gen-1", first)
	}
	if !first.DecidedAt.Equal(created) {
		t.Errorf("first decided_at = %v, want %v", first.DecidedAt, created)
	}
	if !rows.closed {
		t.Error("cursor left open after the scan")
	}

	ragged := &stubInputInvalidRows{records: [][]any{{"only-one-column"}}}
	if _, err := scanInputInvalidFacts(ragged); err == nil {
		t.Error("ragged row: want a scan failure, got nil")
	}
	failing := &stubInputInvalidRows{err: context.DeadlineExceeded}
	if _, err := scanInputInvalidFacts(failing); err == nil {
		t.Error("cursor error: want the failure, got nil")
	}
}

// stubInputInvalidPool binds canned quarantine rows or a canned query
// error behind the adminread Pool seam: ListReducerInputInvalidFacts
// success and query-failure legs without a live database.
type stubInputInvalidPool struct {
	rows   pgx.Rows
	err    error
	closed bool
}

func (p *stubInputInvalidPool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.rows, nil
}

func (p *stubInputInvalidPool) Close() { p.closed = true }

// TestListReducerInputInvalidFactsThroughPool pins the store read
// path: success folds rows and releases the cursor, a query failure
// surfaces, and a nil or pool-less store fails closed.
func TestListReducerInputInvalidFactsThroughPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	created := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	rows := &stubInputInvalidRows{records: [][]any{
		{"fact-1", "typed-payload", "spec_replicas", "input_invalid", "deploy", "scope-1", "gen-1", created},
	}}
	store := newStore(&stubInputInvalidPool{rows: rows})
	items, err := store.ListReducerInputInvalidFacts(ctx, apilibadmin.InputInvalidFactListFilter{
		ScopeID:      "scope-1",
		GenerationID: "gen-1",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].FactID != "fact-1" {
		t.Fatalf("items = %+v, want the one row", items)
	}
	if !rows.closed {
		t.Error("cursor left open after the read")
	}

	failing := newStore(&stubInputInvalidPool{err: context.DeadlineExceeded})
	if _, err := failing.ListReducerInputInvalidFacts(ctx, apilibadmin.InputInvalidFactListFilter{}); err == nil {
		t.Error("query failure: want the error, got nil")
	}

	var nilStore *Store
	if _, err := nilStore.ListReducerInputInvalidFacts(ctx, apilibadmin.InputInvalidFactListFilter{}); err == nil {
		t.Error("nil store: want a not-configured error, got nil")
	}
	if _, err := (&Store{}).ListReducerInputInvalidFacts(ctx, apilibadmin.InputInvalidFactListFilter{}); err == nil {
		t.Error("closed store: want a not-configured error, got nil")
	}
}
