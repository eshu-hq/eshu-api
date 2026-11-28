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

// TestBuildGetReindexStateQueryPinsFleetRow pins the fleet watermark
// SELECT against the core reindex read: the same table, the same
// requested_at column, one bind marker for the ingester.
func TestBuildGetReindexStateQueryPinsFleetRow(t *testing.T) {
	t.Parallel()

	query, args := buildGetReindexStateQuery("repository")
	for _, fragment := range []string{
		"FROM runtime_ingester_control",
		"reindex_request_requested_at",
		"ingester = $1",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query lacks fragment %q:\n%s", fragment, query)
		}
	}
	if len(args) != 1 || args[0] != "repository" {
		t.Errorf("args = %v, want the repository ingester", args)
	}
	if strings.Contains(query, "INSERT") || strings.Contains(query, "UPDATE") {
		t.Errorf("fleet query writes:\n%s", query)
	}
}

// TestBuildRepositoryReindexRequestedAtQueryPinsPointRead pins the
// per-repository watermark SELECT: the same table and columns as the
// core point read, bounded by the request through ANY.
func TestBuildRepositoryReindexRequestedAtQueryPinsPointRead(t *testing.T) {
	t.Parallel()

	query, args := buildRepositoryReindexRequestedAtQuery([]string{"s1", "s2"})
	for _, fragment := range []string{
		"FROM repository_reindex_requests",
		"scope_id",
		"requested_at",
		"scope_id = ANY($1",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query lacks fragment %q:\n%s", fragment, query)
		}
	}
	if len(args) != 1 {
		t.Fatalf("args = %v, want the one scope array", args)
	}
	ids, ok := args[0].([]string)
	if !ok || len(ids) != 2 || ids[0] != "s1" || ids[1] != "s2" {
		t.Errorf("args[0] = %v, want [s1 s2]", args[0])
	}
	if strings.Contains(query, "INSERT") || strings.Contains(query, "UPDATE") {
		t.Errorf("per-repository query writes:\n%s", query)
	}
}

// TestBuildFullReconcileStateQueryPinsMinimalRead pins the
// per-scope progress SELECT: only the three columns the progress
// classification reads (newest activated full ingest time, active
// generation), over the same tables and predicates the core sweep
// uses, with no JSONB operators and no writes.
func TestBuildFullReconcileStateQueryPinsMinimalRead(t *testing.T) {
	t.Parallel()

	query, args := buildFullReconcileStateQuery("s1")
	for _, fragment := range []string{
		"FROM scope_generations",
		"ingested_at",
		"is_delta = false",
		"activated_at IS NOT NULL",
		"FROM ingestion_scopes",
		"active_generation_id",
		"scope_id = $1",
		"LIMIT 1",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query lacks fragment %q:\n%s", fragment, query)
		}
	}
	if len(args) != 1 || args[0] != "s1" {
		t.Errorf("args = %v, want the one scope id", args)
	}
	for _, banned := range []string{"INSERT", "UPDATE", "DELETE", "->>", "payload"} {
		if strings.Contains(query, banned) {
			t.Errorf("scope query carries %q (not a plain column SELECT):\n%s", banned, query)
		}
	}
}

// stubReindexRows drives the reindex scans without a live database:
// one record per programmed row, scanned in the SELECT column order.
type stubReindexRows struct {
	records [][]any
	pos     int
	err     error
	closed  bool
}

func (r *stubReindexRows) Next() bool { return r.pos < len(r.records) }

func (r *stubReindexRows) Scan(dest ...any) error {
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

func (r *stubReindexRows) Err() error { return r.err }

func (r *stubReindexRows) Close() { r.closed = true }

func (r *stubReindexRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *stubReindexRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *stubReindexRows) Values() ([]any, error) { return nil, nil }

func (r *stubReindexRows) RawValues() [][]byte { return nil }

func (r *stubReindexRows) Conn() *pgx.Conn { return nil }

// TestScanProgressRowsFoldsProgressRead pins the three scans: the
// fleet instant, the per-repository watermarks, and the per-scope
// full-generation state (NULL active generation stays empty, NULL
// stored full stays absent).
func TestScanProgressRowsFoldsProgressRead(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

	fleet, err := scanReindexState(&stubReindexRows{records: [][]any{{at}}})
	if err != nil {
		t.Fatalf("fleet scan: %v", err)
	}
	if !fleet.Equal(at) {
		t.Errorf("fleet = %v, want %v", fleet, at)
	}
	if _, err := scanReindexState(&stubReindexRows{records: [][]any{}}); err != nil {
		t.Errorf("absent fleet row: want zero time without error, got %v", err)
	}

	perRepo, err := scanRepositoryWatermarks(&stubReindexRows{records: [][]any{
		{"s1", at},
		{"s2", at.Add(time.Hour)},
	}})
	if err != nil {
		t.Fatalf("per-repository scan: %v", err)
	}
	if len(perRepo) != 2 || !perRepo["s1"].Equal(at) {
		t.Errorf("per-repository = %v, want both watermarks", perRepo)
	}

	state, err := scanFullReconcileState(&stubReindexRows{records: [][]any{
		{sql.NullTime{Time: at, Valid: true}, sql.NullString{String: "g7", Valid: true}},
	}})
	if err != nil {
		t.Fatalf("scope scan: %v", err)
	}
	if !state.HasProjectedFull || !state.LastStoredFullAt.Equal(at) || state.ActiveGenerationID != "g7" {
		t.Errorf("scope state = %+v, want the stored full plus g7", state)
	}
	bare, err := scanFullReconcileState(&stubReindexRows{records: [][]any{
		{sql.NullTime{Valid: false}, sql.NullString{Valid: false}},
	}})
	if err != nil {
		t.Fatalf("bare scope scan: %v", err)
	}
	if bare.HasProjectedFull || bare.ActiveGenerationID != "" {
		t.Errorf("bare scope state = %+v, want absent full and empty generation", bare)
	}
}

// TestScanProgressRowsSurfaceFailures pins the three scans' error
// legs: an iteration error with no rows, a scan-shape failure, and a
// trailing error after rows all surface wrapped errors (never silent
// zero values or panics).
func TestScanProgressRowsSurfaceFailures(t *testing.T) {
	t.Parallel()

	boom := context.DeadlineExceeded

	if _, err := scanReindexState(&stubReindexRows{err: boom}); err == nil {
		t.Error("fleet iteration error: want wrapped error, got nil")
	}
	if _, err := scanReindexState(&stubReindexRows{records: [][]any{{time.Now(), "extra-col"}}}); err == nil {
		t.Error("fleet scan-shape failure: want wrapped error, got nil")
	}
	if _, err := scanReindexState(&stubReindexRows{
		records: [][]any{{time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)}},
		err:     boom,
	}); err == nil {
		t.Error("fleet trailing error: want wrapped error, got nil")
	}

	if _, err := scanRepositoryWatermarks(&stubReindexRows{err: boom}); err == nil {
		t.Error("per-repository iteration error: want wrapped error, got nil")
	}
	if _, err := scanRepositoryWatermarks(&stubReindexRows{records: [][]any{{"only-one-col"}}}); err == nil {
		t.Error("per-repository scan-shape failure: want wrapped error, got nil")
	}
	if _, err := scanRepositoryWatermarks(&stubReindexRows{
		records: [][]any{{"s1", time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)}},
		err:     boom,
	}); err == nil {
		t.Error("per-repository trailing error: want wrapped error, got nil")
	}

	if _, err := scanFullReconcileState(&stubReindexRows{err: boom}); err == nil {
		t.Error("scope iteration error: want wrapped error, got nil")
	}
	if _, err := scanFullReconcileState(&stubReindexRows{records: [][]any{{"only-one-col"}}}); err == nil {
		t.Error("scope scan-shape failure: want wrapped error, got nil")
	}
	if _, err := scanFullReconcileState(&stubReindexRows{
		records: [][]any{{sql.NullTime{Valid: false}, sql.NullString{Valid: false}}},
		err:     boom,
	}); err == nil {
		t.Error("scope trailing error: want wrapped error, got nil")
	}
}

// TestMaintenanceStoreThroughPool pins the reader store read path:
// success folds rows and releases cursors, a query failure
// surfaces, and a nil pool fails closed without a query.
func TestMaintenanceStoreThroughPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

	fleetStore := newStore(scriptedReindexPool(at))
	fleet, err := fleetStore.GetReindexState(ctx, "repository")
	if err != nil {
		t.Fatalf("fleet read: %v", err)
	}
	if !fleet.RequestedAt.Equal(at) {
		t.Errorf("fleet requested_at = %v, want %v", fleet.RequestedAt, at)
	}
	perRepo, err := fleetStore.RepositoryReindexRequestedAt(ctx, []string{"s1"})
	if err != nil {
		t.Fatalf("per-repository read: %v", err)
	}
	if !perRepo["s1"].Equal(at) {
		t.Errorf("per-repository = %v, want the s1 watermark", perRepo)
	}
	state, err := fleetStore.FullReconcileState(ctx, "s1")
	if err != nil {
		t.Fatalf("scope read: %v", err)
	}
	if !state.HasProjectedFull || state.ActiveGenerationID != "g7" {
		t.Errorf("scope state = %+v, want the stored full plus g7", state)
	}
	if _, err := fleetStore.FullReconcileState(ctx, "  "); err != nil {
		t.Errorf("blank scope: want the zero state without error, got %v", err)
	}

	failing := newStore(&stubPool{err: context.DeadlineExceeded})
	if _, err := failing.GetReindexState(ctx, "repository"); err == nil {
		t.Error("fleet failure: want the error, got nil")
	}
	if _, err := failing.RepositoryReindexRequestedAt(ctx, []string{"s1"}); err == nil {
		t.Error("per-repository failure: want the error, got nil")
	}
	if _, err := failing.FullReconcileState(ctx, "s1"); err == nil {
		t.Error("scope failure: want the error, got nil")
	}

	var nilStore *Store
	if _, err := nilStore.GetReindexState(ctx, "repository"); err == nil {
		t.Error("nil store fleet: want a not-configured error, got nil")
	}
	if _, err := nilStore.RepositoryReindexRequestedAt(ctx, []string{"s1"}); err == nil {
		t.Error("nil store per-repository: want a not-configured error, got nil")
	}
	if _, err := nilStore.FullReconcileState(ctx, "s1"); err == nil {
		t.Error("nil store scope: want a not-configured error, got nil")
	}
	if _, err := (&Store{}).GetReindexState(ctx, "repository"); err == nil {
		t.Error("pool-less store: want a not-configured error, got nil")
	}
}

// reindexScriptPool serves one canned progress read per query shape
// behind the adminread Pool seam: the fleet instant, the s1
// per-repository watermark, and the s1 scope state. Dispatch is on
// the query text, like a real database routing three statements. It
// emits no spans by design: span evidence must come from the
// production store methods, never from the pool.
type reindexScriptPool struct {
	at time.Time
}

func scriptedReindexPool(at time.Time) Pool {
	return &reindexScriptPool{at: at}
}

func (p *reindexScriptPool) Query(_ context.Context, statement string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(statement, "runtime_ingester_control"):
		return &stubReindexRows{records: [][]any{{p.at}}}, nil
	case strings.Contains(statement, "repository_reindex_requests"):
		return &stubReindexRows{records: [][]any{{"s1", p.at}}}, nil
	default:
		return &stubReindexRows{records: [][]any{{
			sql.NullTime{Time: p.at, Valid: true},
			sql.NullString{String: "g7", Valid: true},
		}}}, nil
	}
}

func (p *reindexScriptPool) Close() {}

// TestMaintenanceStoreSatisfiesApilibPort pins the wire contract:
// the reader store implements the pinned apilib MaintenanceStore, so
// the terminated lane can serve it with no adapter.
func TestMaintenanceStoreSatisfiesApilibPort(t *testing.T) {
	t.Parallel()

	var _ apilibadmin.MaintenanceStore = (*Store)(nil)
}
