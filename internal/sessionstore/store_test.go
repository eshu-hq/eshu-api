// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeRow is a scripted Row: scan copies scripted values or returns err.
type fakeRow struct {
	values []any
	err    error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		if i < len(r.values) {
			switch d := dest[i].(type) {
			case *string:
				*d, _ = r.values[i].(string)
			case *bool:
				*d, _ = r.values[i].(bool)
			case *time.Time:
				*d, _ = r.values[i].(time.Time)
			case *sql.NullTime:
				if t, ok := r.values[i].(time.Time); ok {
					*d = sql.NullTime{Time: t, Valid: true}
				}
			}
		}
	}
	return nil
}

// fakeRows is a scripted multi-row result.
type fakeRows struct {
	rows [][]any
	pos  int
	err  error
}

func (r *fakeRows) Close() error { return nil }

func (r *fakeRows) Err() error { return r.err }

func (r *fakeRows) Next() bool {
	r.pos++
	return r.pos <= len(r.rows)
}

func (r *fakeRows) Scan(dest ...any) error {
	vals := r.rows[r.pos-1]
	if len(vals) < len(dest) {
		return errors.New("sessionstore test: short row cannot fill scan targets")
	}
	for i := range dest {
		if i >= len(vals) {
			break
		}
		switch d := dest[i].(type) {
		case *string:
			*d, _ = vals[i].(string)
		case *bool:
			*d, _ = vals[i].(bool)
		case *time.Time:
			*d, _ = vals[i].(time.Time)
		case *sql.NullTime:
			if t, ok := vals[i].(time.Time); ok {
				*d = sql.NullTime{Time: t, Valid: true}
			}
		}
	}
	return nil
}

type fakeResult struct {
	n   int64
	err error
}

func (r *fakeResult) RowsAffected() (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.n, nil
}

// fakeDB records every call as "handle:op" plus the query text, and
// serves scripted results. The handle name proves replica routing:
// list calls must reach the "replica" handle only when replica reads
// are explicitly enabled.
type fakeDB struct {
	name     string
	calls    *[]string
	queries  *[]string
	row      Row
	rows     Rows
	rowErr   error
	execRes  Result
	execErr  error
	queryErr error
	// probe and probeErr script the -- session-exists-probe row
	// separately from the main statement row. When both are unset
	// the probe reports no row, so legacy ErrNoRows tests classify
	// the deny as unknown without scripting a second result.
	probe    Row
	probeErr error
}

func (f *fakeDB) record(op, q string) {
	*f.calls = append(*f.calls, f.name+":"+op)
	*f.queries = append(*f.queries, q)
}

func (f *fakeDB) QueryRowContext(_ context.Context, q string, _ ...any) Row {
	f.record("queryrow", q)
	if strings.Contains(q, "session-exists-probe") {
		if f.probeErr != nil {
			return &fakeRow{err: f.probeErr}
		}
		if f.probe != nil {
			return f.probe
		}
		return &fakeRow{err: sql.ErrNoRows}
	}
	if f.rowErr != nil {
		return &fakeRow{err: f.rowErr}
	}
	return f.row
}

func (f *fakeDB) QueryContext(_ context.Context, q string, _ ...any) (Rows, error) {
	f.record("query", q)
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.rows, nil
}

func (f *fakeDB) ExecContext(_ context.Context, q string, _ ...any) (Result, error) {
	f.record("exec", q)
	if f.execErr != nil {
		return nil, f.execErr
	}
	return f.execRes, nil
}

func newFakePair() (primary *fakeDB, replica *fakeDB, calls *[]string, queries *[]string) {
	calls = &[]string{}
	queries = &[]string{}
	primary = &fakeDB{name: "primary", calls: calls, queries: queries}
	replica = &fakeDB{name: "replica", calls: calls, queries: queries}
	return primary, replica, calls, queries
}

func TestResolve_EmptyHashUnresolvedWithoutDBCall(t *testing.T) {
	t.Parallel()
	primary, replica, calls, _ := newFakePair()
	s := NewStore(primary, WithReplica(replica, true))
	_, ok, err := s.ResolveBrowserSession(context.Background(), "", "csrf", true, time.Now())
	if err != nil {
		t.Fatalf("empty hash must deny without error, got %v", err)
	}
	if ok {
		t.Fatal("empty hash must resolve ok=false")
	}
	if len(*calls) != 0 {
		t.Fatalf("empty hash must not touch the database, got %v", *calls)
	}
	if got := s.Stats().Unresolved.Load(); got != 1 {
		t.Fatalf("unresolved counter = %d, want 1", got)
	}
}

func TestResolve_RefreshesIdleExpiryOnPrimary(t *testing.T) {
	t.Parallel()
	primary, replica, _, queries := newFakePair()
	now := time.Now().UTC().Truncate(time.Second)
	primary.row = &fakeRow{values: []any{"subj-hash", "tenant-a", "ws-1", true, "csrf-hash", now}}
	s := NewStore(primary, WithReplica(replica, true))
	got, ok, err := s.ResolveBrowserSession(context.Background(), "sess-hash", "csrf-hash", true, now)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !ok {
		t.Fatal("resolve must return ok=true for a live row")
	}
	if got.SubjectIDHash != "subj-hash" || !got.AllScopes {
		t.Fatalf("unexpected auth context: %+v", got)
	}
	q := (*queries)[0]
	if !strings.Contains(strings.ToUpper(q), "UPDATE") {
		t.Fatalf("resolve must UPDATE last_seen/idle expiry, got: %s", q)
	}
	if strings.Contains(q, "sess-hash") {
		t.Fatalf("hash must travel as an argument, never interpolated: %s", q)
	}
}

func TestResolve_UnknownHashDeniesWithoutError(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	primary.rowErr = sql.ErrNoRows
	s := NewStore(primary)
	_, ok, err := s.ResolveBrowserSession(context.Background(), "missing", "", false, time.Now())
	if err != nil {
		t.Fatalf("unknown hash must deny without error, got %v", err)
	}
	if ok {
		t.Fatal("unknown hash must resolve ok=false")
	}
	if got := s.Stats().Unresolved.Load(); got != 1 {
		t.Fatalf("unresolved counter = %d, want 1", got)
	}
}

func TestResolve_DBErrorSurfacesAsError(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	boom := errors.New("primary unreachable")
	primary.rowErr = boom
	s := NewStore(primary)
	_, _, err := s.ResolveBrowserSession(context.Background(), "sess-hash", "", false, time.Now())
	if !errors.Is(err, boom) {
		t.Fatalf("store error must propagate (caller denies 503), got %v", err)
	}
	if got := s.Stats().StoreErrors.Load(); got != 1 {
		t.Fatalf("store-error counter = %d, want 1", got)
	}
}
