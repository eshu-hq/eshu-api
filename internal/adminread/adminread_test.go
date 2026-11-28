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

// TestLoadConfigFromEnvFailsClosed pins the bounded DSN config: an
// empty DSN refuses to start, an explicit MaxConns must be a positive
// integer inside the census budget, and the default keeps the
// validated ceiling.
func TestLoadConfigFromEnvFailsClosed(t *testing.T) {
	t.Parallel()

	getenv := func(vars map[string]string) func(string) string {
		return func(key string) string { return vars[key] }
	}

	if _, err := LoadConfigFromEnv(getenv(nil)); err == nil {
		t.Error("empty DSN: want a refusal, got nil")
	}
	if _, err := LoadConfigFromEnv(getenv(map[string]string{AdminReadDSNEnv: "  "})); err == nil {
		t.Error("blank DSN: want a refusal, got nil")
	}
	for _, raw := range []string{"0", "-3", "many", "5.5"} {
		vars := map[string]string{AdminReadDSNEnv: "postgres://reader:secret@db:5432/eshu", AdminReadMaxConnsEnv: raw}
		if _, err := LoadConfigFromEnv(getenv(vars)); err == nil {
			t.Errorf("MaxConns %q: want a refusal, got nil", raw)
		}
	}
	vars := map[string]string{
		AdminReadDSNEnv:      "postgres://reader:secret@db:5432/eshu",
		AdminReadMaxConnsEnv: "1000",
	}
	if _, err := LoadConfigFromEnv(getenv(vars)); err == nil {
		t.Error("oversized MaxConns: want a budget refusal, got nil")
	}

	cfg, err := LoadConfigFromEnv(getenv(map[string]string{AdminReadDSNEnv: "postgres://reader:secret@db:5432/eshu"}))
	if err != nil {
		t.Fatalf("default config: %v", err)
	}
	if cfg.MaxConns != DefaultMaxConns {
		t.Errorf("default MaxConns = %d, want %d", cfg.MaxConns, DefaultMaxConns)
	}
	cfg, err = LoadConfigFromEnv(getenv(map[string]string{
		AdminReadDSNEnv:      "postgres://reader:secret@db:5432/eshu",
		AdminReadMaxConnsEnv: "3",
	}))
	if err != nil {
		t.Fatalf("explicit MaxConns: %v", err)
	}
	if cfg.MaxConns != 3 {
		t.Errorf("MaxConns = %d, want 3", cfg.MaxConns)
	}
}

// TestNewStoreFailsClosed pins every non-live open failure: an
// empty DSN, an oversized ceiling, an unparsable DSN, and an
// unreachable host all refuse without a live database. The success
// path is the live proof's (env-gated).
func TestNewStoreFailsClosed(t *testing.T) {
	// No t.Parallel: the nil-getenv leg pins the process environment
	// through t.Setenv, which parallel tests must not touch.
	t.Setenv(AdminReadDSNEnv, "")
	t.Setenv(AdminReadMaxConnsEnv, "")

	ctx := context.Background()

	if _, err := NewStore(ctx, Config{}); err == nil {
		t.Error("empty DSN: want a refusal, got nil")
	}
	if _, err := NewStore(ctx, Config{DSN: "postgres://reader:secret@db:5432/eshu", MaxConns: 1000}); err == nil {
		t.Error("oversized ceiling: want a budget refusal, got nil")
	}
	if _, err := NewStore(ctx, Config{DSN: "://missing-scheme-host"}); err == nil {
		t.Error("unparsable DSN: want a refusal, got nil")
	}
	if _, err := NewStore(ctx, Config{DSN: "postgres://127.0.0.1:1/eshu?sslmode=disable"}); err == nil {
		t.Error("unreachable host: want a refusal, got nil")
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := NewStore(canceled, Config{DSN: "postgres://reader:secret@db:5432/eshu"}); err == nil {
		t.Error("canceled open: want a refusal, got nil")
	}
	if _, err := LoadConfigFromEnv(nil); err == nil {
		t.Error("nil getenv: want a refusal (no DSN in process env), got nil")
	}

	var nilStore *Store
	nilStore.Close()
	if _, err := nilStore.ListWorkItems(ctx, apilibadmin.WorkItemFilter{}); err == nil {
		t.Error("nil store: want a not-configured error, got nil")
	}
	if _, err := (&Store{}).ListWorkItems(ctx, apilibadmin.WorkItemFilter{}); err == nil {
		t.Error("closed store: want a not-configured error, got nil")
	}
}

// TestScanWorkItemsRowsError pins the cursor-error leg: a failing
// cursor surfaces instead of silent partial rows.
func TestScanWorkItemsRowsError(t *testing.T) {
	t.Parallel()

	rows := &stubWorkItemRows{err: context.DeadlineExceeded}
	if _, err := scanWorkItems(rows); err == nil {
		t.Error("cursor error: want the failure, got nil")
	}
	ragged := &stubWorkItemRows{records: [][]any{{"only-one-column"}}}
	if _, err := scanWorkItems(ragged); err == nil {
		t.Error("ragged row: want a scan failure, got nil")
	}
}

// TestReaderBudgetDefaultValidates pins the census-recorded replica
// line: the default ceiling fits the server with room to spare, and
// the formula rejects an unmeasured server.
func TestReaderBudgetDefaultValidates(t *testing.T) {
	t.Parallel()

	budget := ReaderBudget(DefaultMaxConns)
	if err := budget.Validate(); err != nil {
		t.Fatalf("default budget: %v", err)
	}
	if got := budget.Total(); got != 20 {
		t.Errorf("default budget total = %d, want 20 (5 x (1 + 1) + 0 + 10)", got)
	}
	if want := "PerReplicaMaxConns x (MaxReplicas + SurgeReplicas) + CoreAPIBudget + Headroom <= ServerMaxConnections"; budget.Formula() != want {
		t.Errorf("budget formula = %q, want the pinned arithmetic", budget.Formula())
	}
	oversized := ReaderBudget(1000)
	if err := oversized.Validate(); err == nil {
		t.Error("oversized budget: want a refusal, got nil")
	}
}

// TestBuildListWorkItemsQueryPinsCoreColumns pins the reader SELECT
// against the core column set, order, and bound: same columns in the
// same order, the same default limit, one bind marker per bound
// selector, and no clause for absent selectors.
func TestBuildListWorkItemsQueryPinsCoreColumns(t *testing.T) {
	t.Parallel()

	query, args := buildListWorkItemsQuery(apilibadmin.WorkItemFilter{
		Statuses:     []string{"pending", "failed"},
		ScopeID:      "s1",
		Stage:        " build ",
		FailureClass: " fc ",
		Limit:        7,
	})

	for _, column := range []string{
		"work_item_id", "scope_id", "generation_id", "stage", "domain",
		"status", "attempt_count", "replay_attempt_baseline", "lease_owner",
		"failure_class", "failure_message", "created_at", "updated_at",
		"visible_at", "failure_details",
	} {
		if !strings.Contains(query, column) {
			t.Errorf("query lacks column %s:\n%s", column, query)
		}
	}
	if !strings.Contains(query, "FROM fact_work_items") {
		t.Errorf("query lacks the fact_work_items source:\n%s", query)
	}
	for _, clause := range []string{
		"status = ANY($1)", "scope_id = $2", "stage = $3", "failure_class = $4",
		"ORDER BY updated_at DESC, work_item_id ASC LIMIT $5",
	} {
		if !strings.Contains(query, clause) {
			t.Errorf("query lacks clause %q:\n%s", clause, query)
		}
	}
	if len(args) != 5 {
		t.Fatalf("args = %v, want 5 bound values", args)
	}
	if got, ok := args[0].([]string); !ok || len(got) != 2 || got[0] != "pending" || got[1] != "failed" {
		t.Errorf("args[0] = %v, want the statuses slice", args[0])
	}
	if args[1] != "s1" || args[2] != "build" || args[3] != "fc" || args[4] != 7 {
		t.Errorf("args = %v, want trimmed selectors with limit 7", args)
	}

	bare, bareArgs := buildListWorkItemsQuery(apilibadmin.WorkItemFilter{})
	if !strings.Contains(bare, "LIMIT $1") {
		t.Errorf("bare query lacks the LIMIT bind marker:\n%s", bare)
	}
	if len(bareArgs) != 1 || bareArgs[0] != 100 {
		t.Errorf("bare args = %v, want the default limit 100", bareArgs)
	}
	for _, fragment := range []string{"ANY($", "scope_id = $", "stage = $", "failure_class = $"} {
		if strings.Contains(strings.SplitN(bare, "ORDER BY", 2)[0], fragment) {
			t.Errorf("bare query carries a selector clause %q:\n%s", fragment, bare)
		}
	}
}

// stubWorkItemRows drives scanWorkItems without a live database: one
// row per programmed record, scanned in the SELECT column order.
type stubWorkItemRows struct {
	records [][]any
	pos     int
	err     error
	closed  bool
}

func (r *stubWorkItemRows) Next() bool { return r.pos < len(r.records) }

func (r *stubWorkItemRows) Scan(dest ...any) error {
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

func (r *stubWorkItemRows) Err() error { return r.err }

func (r *stubWorkItemRows) Close() { r.closed = true }

// The pgx.Rows remainder: the scan only drives Next/Scan/Err/Close,
// so the rest stay inert.
func (r *stubWorkItemRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *stubWorkItemRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *stubWorkItemRows) Values() ([]any, error) { return nil, nil }

func (r *stubWorkItemRows) RawValues() [][]byte { return nil }

func (r *stubWorkItemRows) Conn() *pgx.Conn { return nil }

// stubPool binds canned rows or a canned query error behind the pool
// seam: ListWorkItems success and query-failure legs without a live
// database.
type stubPool struct {
	rows   *stubWorkItemRows
	err    error
	closed bool
}

func (p *stubPool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.rows, nil
}

func (p *stubPool) Close() { p.closed = true }

// TestListWorkItemsThroughPool pins the store read path: success
// folds rows and releases the cursor, a query failure surfaces, and
// Close drains the pool.
func TestListWorkItemsThroughPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	created := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	rows := &stubWorkItemRows{records: [][]any{
		{
			"w1", "s1", "g1", "stage", "d", "pending", 2, 0,
			sql.NullString{Valid: false},
			sql.NullString{Valid: false},
			sql.NullString{Valid: false},
			created, created,
			sql.NullTime{Valid: false},
			sql.NullString{Valid: false},
		},
	}}
	store := newStore(&stubPool{rows: rows})
	items, err := store.ListWorkItems(ctx, apilibadmin.WorkItemFilter{Limit: 3})
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
	if _, err := failing.ListWorkItems(ctx, apilibadmin.WorkItemFilter{}); err == nil {
		t.Error("query failure: want the error, got nil")
	}

	pool := &stubPool{}
	bound := newStore(pool)
	bound.Close()
	if !pool.closed {
		t.Error("Close left the pool open")
	}
}

// TestScanWorkItemsFoldsDetails pins the scan against the core fold:
// nullable columns stay nil when NULL, present values map through,
// and failure_details unfolds into operator_note/prior_failure.
func TestScanWorkItemsFoldsDetails(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	updated := created.Add(time.Hour)
	visible := created.Add(2 * time.Hour)
	rows := &stubWorkItemRows{records: [][]any{
		{
			"w1", "s1", "g1", "build", "d", "pending", 3, 1,
			sql.NullString{String: "owner-1", Valid: true},
			sql.NullString{String: "build_failed", Valid: true},
			sql.NullString{String: "boom", Valid: true},
			created, updated,
			sql.NullTime{Time: visible, Valid: true},
			sql.NullString{String: `{"operator_note":"look again"}`, Valid: true},
		},
		{
			"w2", "s1", "g1", "build", "d", "failed", 1, 0,
			sql.NullString{Valid: false},
			sql.NullString{Valid: false},
			sql.NullString{Valid: false},
			created, updated,
			sql.NullTime{Valid: false},
			sql.NullString{Valid: false},
		},
	}}

	items, err := scanWorkItems(rows)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	// The caller owns the cursor lifetime (ListWorkItems defers the
	// close); the scan only folds what Next yields.
	rows.Close()
	if !rows.closed {
		t.Error("stub rows not marked closed")
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	first := items[0]
	if first.WorkItemID != "w1" || first.AttemptCount != 3 || first.ReplayAttemptBaseline != 1 {
		t.Errorf("first row identity = %+v, want w1/3/1", first)
	}
	if first.LeaseOwner == nil || *first.LeaseOwner != "owner-1" {
		t.Errorf("first lease_owner = %v, want owner-1", first.LeaseOwner)
	}
	if first.FailureClass == nil || *first.FailureClass != "build_failed" {
		t.Errorf("first failure_class = %v, want build_failed", first.FailureClass)
	}
	if first.FailureMessage == nil || *first.FailureMessage != "boom" {
		t.Errorf("first failure_message = %v, want boom", first.FailureMessage)
	}
	if first.OperatorNote == nil || *first.OperatorNote != "look again" {
		t.Errorf("first operator_note = %v, want the unfolded note", first.OperatorNote)
	}
	if first.VisibleAt == nil || !first.VisibleAt.Equal(visible) {
		t.Errorf("first visible_at = %v, want %v", first.VisibleAt, visible)
	}
	second := items[1]
	if second.LeaseOwner != nil || second.FailureClass != nil || second.FailureMessage != nil ||
		second.OperatorNote != nil || second.PriorFailure != nil || second.VisibleAt != nil {
		t.Errorf("second row keeps nil pointers: %+v", second)
	}
}

// TestApplyWorkItemDetailsParity pins the failure_details fold: a
// malformed prior_failure keeps a valid operator_note, non-JSON or
// empty folds stay nil, and RFC3339Nano instants normalize to
// RFC3339.
func TestApplyWorkItemDetailsParity(t *testing.T) {
	t.Parallel()

	t.Run("note survives malformed prior", func(t *testing.T) {
		t.Parallel()
		var item apilibadmin.WorkItem
		applyWorkItemDetails(&item, sql.NullString{
			String: `{"operator_note":"keep me","prior_failure":{"status":42}}`,
			Valid:  true,
		})
		if item.OperatorNote == nil || *item.OperatorNote != "keep me" {
			t.Errorf("operator_note = %v, want keep me", item.OperatorNote)
		}
		if item.PriorFailure != nil {
			t.Errorf("prior_failure = %v, want nil", item.PriorFailure)
		}
	})

	t.Run("non-JSON stays nil", func(t *testing.T) {
		t.Parallel()
		for _, raw := range []string{"", "  ", "not json", "[1,2]"} {
			var item apilibadmin.WorkItem
			applyWorkItemDetails(&item, sql.NullString{String: raw, Valid: true})
			if item.OperatorNote != nil || item.PriorFailure != nil {
				t.Errorf("raw %q unfolded: %+v", raw, item)
			}
		}
		var item apilibadmin.WorkItem
		applyWorkItemDetails(&item, sql.NullString{Valid: false})
		if item.OperatorNote != nil || item.PriorFailure != nil {
			t.Errorf("NULL unfolded: %+v", item)
		}
	})

	t.Run("prior normalizes and empty prior stays nil", func(t *testing.T) {
		t.Parallel()
		var item apilibadmin.WorkItem
		applyWorkItemDetails(&item, sql.NullString{
			String: `{"prior_failure":{"status":"failed","failure_class":"build_failed","failure_message":"boom","updated_at":"2026-03-04T05:06:07.5Z"}}`,
			Valid:  true,
		})
		if item.PriorFailure == nil {
			t.Fatal("prior_failure is nil, want the kept failure")
		}
		if item.PriorFailure.Status != "failed" || item.PriorFailure.FailureClass != "build_failed" ||
			item.PriorFailure.FailureMessage != "boom" || item.PriorFailure.UpdatedAt != "2026-03-04T05:06:07Z" {
			t.Errorf("prior_failure = %+v, want trimmed fields with RFC3339 instant", item.PriorFailure)
		}

		var empty apilibadmin.WorkItem
		applyWorkItemDetails(&empty, sql.NullString{String: `{"prior_failure":{}}`, Valid: true})
		if empty.PriorFailure != nil {
			t.Errorf("empty prior unfolded: %+v", empty.PriorFailure)
		}
	})
}
