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
		}
	}
	return nil
}

type fakeResult struct{ n int64 }

func (r *fakeResult) RowsAffected() (int64, error) { return r.n, nil }

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
}

func (f *fakeDB) record(op, q string) {
	*f.calls = append(*f.calls, f.name+":"+op)
	*f.queries = append(*f.queries, q)
}

func (f *fakeDB) QueryRowContext(_ context.Context, q string, _ ...any) Row {
	f.record("queryrow", q)
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

func TestResolve_CSRFMismatchDenies(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	primary.row = &fakeRow{values: []any{"subj-hash", "tenant-a", "ws-1", true, "other-csrf", time.Now()}}
	s := NewStore(primary)
	_, ok, err := s.ResolveBrowserSession(context.Background(), "sess-hash", "csrf-hash", true, time.Now())
	if err != nil {
		t.Fatalf("CSRF mismatch must deny without error, got %v", err)
	}
	if ok {
		t.Fatal("CSRF mismatch must resolve ok=false")
	}
	if got := s.Stats().CSRFMismatches.Load(); got != 1 {
		t.Fatalf("csrf counter = %d, want 1", got)
	}
}

func TestRevoke_IsIdempotentOnZeroRows(t *testing.T) {
	t.Parallel()
	primary, _, _, queries := newFakePair()
	primary.execRes = &fakeResult{n: 0}
	s := NewStore(primary)
	if err := s.RevokeBrowserSession(context.Background(), "sess-hash", time.Now()); err != nil {
		t.Fatalf("revoke of an already-gone session must succeed, got %v", err)
	}
	q := (*queries)[0]
	if !strings.Contains(strings.ToUpper(q), "UPDATE") {
		t.Fatalf("revoke must UPDATE revoked_at, never DELETE: %s", q)
	}
	if strings.Contains(strings.ToUpper(q), "DELETE") {
		t.Fatalf("revoke must never DELETE: %s", q)
	}
}

func TestSwitch_NonAllScopesRowDenies(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	primary.rowErr = sql.ErrNoRows
	s := NewStore(primary)
	_, ok, err := s.SwitchBrowserSessionWorkspace(context.Background(), "scoped-hash", "t", "w", time.Now())
	if err != nil {
		t.Fatalf("scoped switch must deny without error, got %v", err)
	}
	if ok {
		t.Fatal("scoped session switch must resolve ok=false")
	}
}

func TestList_MetadataOnlyAndReplicaRouted(t *testing.T) {
	t.Parallel()
	primary, replica, calls, queries := newFakePair()
	now := time.Now().UTC()
	replica.rows = &fakeRows{rows: [][]any{{"tenant-a", "ws-1", now, now, now, now, true}}}
	s := NewStore(primary, WithReplica(replica, true))
	items, err := s.ListSessionsBySubject(context.Background(), "subj-hash", now, "sess-hash", 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || !items[0].Current {
		t.Fatalf("unexpected list items: %+v", items)
	}
	if len(*calls) != 1 || (*calls)[0] != "replica:query" {
		t.Fatalf("metadata list must ride the replica when enabled, got %v", *calls)
	}
	// The Current flag is computed server-side by hash comparison, so
	// the hash name legitimately appears inside that single computed
	// expression and past FROM. Strip the parenthesized computation
	// first: a bare hash column anywhere else must fail.
	selectList := strings.ToUpper((*queries)[0])
	if idx := strings.Index(selectList, "FROM"); idx >= 0 {
		selectList = selectList[:idx]
	}
	if start := strings.Index(selectList, "("); start >= 0 {
		if end := strings.Index(selectList[start:], ")"); end >= 0 {
			selectList = selectList[:start] + selectList[start+end+1:]
		}
	}
	for _, forbidden := range []string{"SESSION_HASH", "CSRF_HASH", "TOKEN_HASH"} {
		if strings.Contains(selectList, forbidden) {
			t.Fatalf("metadata list must never select %s: %s", forbidden, (*queries)[0])
		}
	}
}

func TestList_PrimaryByDefault(t *testing.T) {
	t.Parallel()
	primary, replica, calls, _ := newFakePair()
	primary.rows = &fakeRows{}
	s := NewStore(primary, WithReplica(replica, false))
	if _, err := s.ListSessionsBySubject(context.Background(), "subj", time.Now(), "", 50, 0); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(*calls) != 1 || (*calls)[0] != "primary:query" {
		t.Fatalf("lists must default to the primary, got %v", *calls)
	}
}

func TestTokenRotate_ScopesByOwnership(t *testing.T) {
	t.Parallel()
	primary, _, _, queries := newFakePair()
	primary.execRes = &fakeResult{n: 1}
	s := NewStore(primary)
	err := s.RotateLocalIdentityAPIToken(context.Background(), APITokenRotate{
		TokenID: "tok-1", SubjectIDHash: "subj-hash", NewTokenHash: "new-hash",
	})
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	q := strings.ToUpper((*queries)[0])
	if !strings.Contains(q, "UPDATE") {
		t.Fatalf("rotate must be a single UPDATE, got: %s", (*queries)[0])
	}
	if !strings.Contains((*queries)[0], "subject") {
		t.Fatalf("rotate must scope by token ownership: %s", (*queries)[0])
	}
}

func TestIdentityReads_AreSelectOnly(t *testing.T) {
	t.Parallel()
	primary, _, _, queries := newFakePair()
	primary.rows = &fakeRows{}
	primary.row = &fakeRow{values: []any{true, time.Now()}}
	s := NewStore(primary)
	if _, err := s.ListAPITokensBySubject(context.Background(), "subj", time.Now()); err != nil {
		t.Fatalf("token list: %v", err)
	}
	if _, err := s.GetLocalIdentityMFAStatus(context.Background(), "subj", time.Now()); err != nil {
		t.Fatalf("mfa status: %v", err)
	}
	for _, q := range *queries {
		upper := strings.ToUpper(q)
		if !strings.HasPrefix(strings.TrimSpace(upper), "SELECT") {
			t.Fatalf("identity/token reads must be SELECT-only, got: %s", q)
		}
		if strings.Contains(upper, "TOKEN_HASH") && strings.Contains(upper, "SELECT") && strings.Contains(upper, "FROM") {
			t.Fatalf("token list must never return token_hash: %s", q)
		}
	}
}
