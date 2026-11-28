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

// TestBuildListDeadLetterWorkItemsQueryPinsCoreColumns pins the
// reader SELECT against the core dead-letter read: the same columns
// in the same order, the same dead_letter predicate, the same
// filters, the same default limit, and one bind marker per bound
// selector.
func TestBuildListDeadLetterWorkItemsQueryPinsCoreColumns(t *testing.T) {
	t.Parallel()

	query, args := buildListDeadLetterWorkItemsQuery(apilibadmin.DeadLetterListFilter{
		FailureClass: " fc ",
		Domain:       " d ",
		ScopeID:      "s1",
		Limit:        7,
	})

	for _, column := range []string{
		"work_item_id", "scope_id", "generation_id", "stage", "domain",
		"collector_kind", "attempt_count", "failure_class",
		"created_at", "updated_at", "visible_at",
	} {
		if !strings.Contains(query, column) {
			t.Errorf("query lacks column %s:\n%s", column, query)
		}
	}
	for _, fragment := range []string{
		"FROM fact_work_items AS work",
		"JOIN ingestion_scopes AS scope",
		"work.status = 'dead_letter'",
		"scope.collector_kind",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query lacks fragment %q:\n%s", fragment, query)
		}
	}
	for _, clause := range []string{
		"work.failure_class = $1", "work.domain = $2", "work.scope_id = $3",
		"ORDER BY work.updated_at DESC, work.work_item_id ASC LIMIT $4",
	} {
		if !strings.Contains(query, clause) {
			t.Errorf("query lacks clause %q:\n%s", clause, query)
		}
	}
	if len(args) != 4 {
		t.Fatalf("args = %v, want 4 bound values", args)
	}
	if args[0] != "fc" || args[1] != "d" || args[2] != "s1" || args[3] != 7 {
		t.Errorf("args = %v, want trimmed selectors with limit 7", args)
	}

	bare, bareArgs := buildListDeadLetterWorkItemsQuery(apilibadmin.DeadLetterListFilter{})
	if !strings.Contains(bare, "LIMIT $1") {
		t.Errorf("bare query lacks the LIMIT bind marker:\n%s", bare)
	}
	if len(bareArgs) != 1 || bareArgs[0] != 100 {
		t.Errorf("bare args = %v, want the default limit 100", bareArgs)
	}
	for _, fragment := range []string{"work.failure_class = $", "work.domain = $", "work.scope_id = $", "scope.collector_kind = $"} {
		if strings.Contains(strings.SplitN(bare, "ORDER BY", 2)[0], fragment) {
			t.Errorf("bare query carries a selector clause %q:\n%s", fragment, bare)
		}
	}
}

// TestBuildListDeadLetterWorkItemsQueryCollectorKind pins the
// collector_kind leg of the join: the bound value lands on the
// scope-side predicate.
func TestBuildListDeadLetterWorkItemsQueryCollectorKind(t *testing.T) {
	t.Parallel()

	query, args := buildListDeadLetterWorkItemsQuery(apilibadmin.DeadLetterListFilter{
		CollectorKind: " ck ",
		Limit:         10,
	})
	if !strings.Contains(query, "scope.collector_kind = $1") {
		t.Errorf("query lacks the scope-side collector_kind clause:\n%s", query)
	}
	if len(args) != 2 || args[0] != "ck" || args[1] != 10 {
		t.Errorf("args = %v, want [ck 10]", args)
	}
}

// TestBuildListDeadLetterWorkItemsQueryWindow pins the core-exact
// updated_at range: after inclusive (>=, the at-after row stays),
// before exclusive (<, the at-before row drops), threaded as bind
// args in core order (after CollectorKind, before the access clause
// and the LIMIT).
func TestBuildListDeadLetterWorkItemsQueryWindow(t *testing.T) {
	t.Parallel()

	after := time.Date(2026, 3, 4, 6, 6, 7, 0, time.UTC)
	before := time.Date(2026, 3, 4, 8, 6, 7, 0, time.UTC)
	query, args := buildListDeadLetterWorkItemsQuery(apilibadmin.DeadLetterListFilter{
		CollectorKind: "ck",
		UpdatedAfter:  &after,
		UpdatedBefore: &before,
		Limit:         10,
	})
	for _, clause := range []string{
		"scope.collector_kind = $1",
		"work.updated_at >= $2",
		"work.updated_at < $3",
		"ORDER BY work.updated_at DESC, work.work_item_id ASC LIMIT $4",
	} {
		if !strings.Contains(query, clause) {
			t.Errorf("query lacks clause %q:\n%s", clause, query)
		}
	}
	if len(args) != 4 {
		t.Fatalf("args = %v, want 4 bound values", args)
	}
	if args[0] != "ck" || args[3] != 10 {
		t.Errorf("args = %v, want [ck after before 10]", args)
	}
	if got, ok := args[1].(time.Time); !ok || !got.Equal(after) {
		t.Errorf("args[1] = %v, want the after bound", args[1])
	}
	if got, ok := args[2].(time.Time); !ok || !got.Equal(before) {
		t.Errorf("args[2] = %v, want the before bound", args[2])
	}

	afterOnly, afterOnlyArgs := buildListDeadLetterWorkItemsQuery(apilibadmin.DeadLetterListFilter{
		UpdatedAfter: &after,
	})
	if !strings.Contains(afterOnly, "work.updated_at >= $1") {
		t.Errorf("after-only query lacks the >= clause:\n%s", afterOnly)
	}
	if strings.Contains(strings.SplitN(afterOnly, "ORDER BY", 2)[0], "work.updated_at < $") {
		t.Errorf("after-only query carries a before clause:\n%s", afterOnly)
	}
	if len(afterOnlyArgs) != 2 {
		t.Errorf("after-only args = %v, want the bound plus the default limit", afterOnlyArgs)
	}

	beforeOnly, beforeOnlyArgs := buildListDeadLetterWorkItemsQuery(apilibadmin.DeadLetterListFilter{
		UpdatedBefore: &before,
	})
	if !strings.Contains(beforeOnly, "work.updated_at < $1") {
		t.Errorf("before-only query lacks the < clause:\n%s", beforeOnly)
	}
	if strings.Contains(strings.SplitN(beforeOnly, "ORDER BY", 2)[0], "work.updated_at >= $") {
		t.Errorf("before-only query carries an after clause:\n%s", beforeOnly)
	}
	if len(beforeOnlyArgs) != 2 {
		t.Errorf("before-only args = %v, want the bound plus the default limit", beforeOnlyArgs)
	}
}

// TestBuildListDeadLetterWorkItemsQueryAccessClause pins the scoped
// seam: granted repositories/scopes add the core-identical access
// clause, and the empty-grant (shared-key, all-scopes) query carries
// no restriction.
func TestBuildListDeadLetterWorkItemsQueryAccessClause(t *testing.T) {
	t.Parallel()

	granted, args := buildListDeadLetterWorkItemsQuery(apilibadmin.DeadLetterListFilter{
		AllowedRepositoryIDs: []string{"repo-1"},
		AllowedScopeIDs:      []string{"scope-9"},
		Limit:                10,
	})
	if !strings.Contains(granted, "scope.scope_kind = 'repository'") || !strings.Contains(granted, "work.scope_id = ANY(") {
		t.Errorf("granted query lacks the access clause:\n%s", granted)
	}
	if len(args) != 3 {
		t.Errorf("granted args = %v, want 2 access arrays plus the limit", args)
	}
}

// stubDeadLetterRows drives scanDeadLetters without a live database:
// one row per programmed record, scanned in the SELECT column order.
type stubDeadLetterRows struct {
	records [][]any
	pos     int
	err     error
	closed  bool
}

func (r *stubDeadLetterRows) Next() bool { return r.pos < len(r.records) }

func (r *stubDeadLetterRows) Scan(dest ...any) error {
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
		case *int:
			*slot = value.(int)
		case *time.Time:
			*slot = value.(time.Time)
		case *sql.NullString:
			*slot = value.(sql.NullString)
		case *sql.NullTime:
			*slot = value.(sql.NullTime)
		default:
			return context.Canceled
		}
	}
	return nil
}

func (r *stubDeadLetterRows) Err() error { return r.err }

func (r *stubDeadLetterRows) Close() { r.closed = true }

// The pgx.Rows remainder: the scan only drives Next/Scan/Err/Close,
// so the rest stay inert.
func (r *stubDeadLetterRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *stubDeadLetterRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *stubDeadLetterRows) Values() ([]any, error) { return nil, nil }

func (r *stubDeadLetterRows) RawValues() [][]byte { return nil }

func (r *stubDeadLetterRows) Conn() *pgx.Conn { return nil }

// TestScanDeadLettersFoldsNullables pins the scan against the core
// fold: nullable columns stay nil when NULL and present values map
// through.
func TestScanDeadLettersFoldsNullables(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	updated := created.Add(time.Hour)
	visible := created.Add(2 * time.Hour)
	rows := &stubDeadLetterRows{records: [][]any{
		{
			"w1", "s1", "g1", "build", "d", "collector-1", 3,
			sql.NullString{String: "build_failed", Valid: true},
			created, updated,
			sql.NullTime{Time: visible, Valid: true},
		},
		{
			"w2", "s1", "g1", "build", "d", "collector-1", 1,
			sql.NullString{Valid: false},
			created, updated,
			sql.NullTime{Valid: false},
		},
	}}

	items, err := scanDeadLetters(rows)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	rows.Close()
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	first := items[0]
	if first.WorkItemID != "w1" || first.CollectorKind != "collector-1" || first.AttemptCount != 3 {
		t.Errorf("first row identity = %+v, want w1/collector-1/3", first)
	}
	if first.FailureClass == nil || *first.FailureClass != "build_failed" {
		t.Errorf("first failure_class = %v, want build_failed", first.FailureClass)
	}
	if first.VisibleAt == nil || !first.VisibleAt.Equal(visible) {
		t.Errorf("first visible_at = %v, want %v", first.VisibleAt, visible)
	}
	second := items[1]
	if second.FailureClass != nil || second.VisibleAt != nil {
		t.Errorf("second row keeps nil pointers: %+v", second)
	}
}

// TestScanDeadLettersRowsError pins the cursor-error leg: a failing
// cursor surfaces instead of silent partial rows.
func TestScanDeadLettersRowsError(t *testing.T) {
	t.Parallel()

	rows := &stubDeadLetterRows{err: context.DeadlineExceeded}
	if _, err := scanDeadLetters(rows); err == nil {
		t.Error("cursor error: want the failure, got nil")
	}
	ragged := &stubDeadLetterRows{records: [][]any{{"only-one-column"}}}
	if _, err := scanDeadLetters(ragged); err == nil {
		t.Error("ragged row: want a scan failure, got nil")
	}
}

// TestListDeadLetterWorkItemsThroughPool pins the store read path:
// success folds rows and releases the cursor, a query failure
// surfaces, and Close drains the pool.
func TestListDeadLetterWorkItemsThroughPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	created := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	// The shared stubPool drives stubWorkItemRows, which satisfies
	// the same narrow cursor seam as the dead-letter scan: program
	// it with one 11-column dead-letter record.
	rows := &stubWorkItemRows{records: [][]any{
		{
			"w1", "s1", "g1", "stage", "d", "collector-1", 2,
			sql.NullString{Valid: false},
			created, created,
			sql.NullTime{Valid: false},
		},
	}}
	store := newStore(&stubPool{rows: rows})
	items, err := store.ListDeadLetterWorkItems(ctx, apilibadmin.DeadLetterListFilter{Limit: 3})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].WorkItemID != "w1" {
		t.Fatalf("items = %+v, want the one row", items)
	}
	if !rows.closed {
		t.Error("cursor left open after the read")
	}

	failing := newStore(&stubPool{err: context.DeadlineExceeded})
	if _, err := failing.ListDeadLetterWorkItems(ctx, apilibadmin.DeadLetterListFilter{}); err == nil {
		t.Error("query failure: want the error, got nil")
	}

	var nilStore *Store
	if _, err := nilStore.ListDeadLetterWorkItems(ctx, apilibadmin.DeadLetterListFilter{}); err == nil {
		t.Error("nil store: want a not-configured error, got nil")
	}
	if _, err := (&Store{}).ListDeadLetterWorkItems(ctx, apilibadmin.DeadLetterListFilter{}); err == nil {
		t.Error("closed store: want a not-configured error, got nil")
	}
}
