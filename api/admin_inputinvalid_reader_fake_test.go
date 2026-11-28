// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// errFakeInputInvalidExhausted fails a Scan that arrives with no row
// staged or the wrong column count: a shape mismatch in the fake,
// never a backend value.
var errFakeInputInvalidExhausted = errors.New("fake input-invalid pool: no row staged for scan")

// fakeInputInvalidRows serves one canned reducer_input_invalid_facts
// reader row in SELECT column order behind pgx.Rows: the P-READ proof
// drives the real adminread store read path (SQL, scan) without a
// live database.
type fakeInputInvalidRows struct {
	staged bool
	closed bool
	at     time.Time
}

func (r *fakeInputInvalidRows) Next() bool { return r.staged }

func (r *fakeInputInvalidRows) Scan(dest ...any) error {
	if !r.staged || len(dest) != 8 {
		return errFakeInputInvalidExhausted
	}
	r.staged = false
	values := []any{
		"fact-1", "typed-payload", "spec_replicas", "input_invalid",
		"deploy", "scope-1", "gen-1",
		r.at,
	}
	for i, value := range values {
		switch slot := dest[i].(type) {
		case *string:
			*slot = value.(string)
		case *time.Time:
			*slot = value.(time.Time)
		default:
			return errFakeInputInvalidExhausted
		}
	}
	return nil
}

func (r *fakeInputInvalidRows) Err() error { return nil }

func (r *fakeInputInvalidRows) Close() { r.closed = true }

// The pgx.Rows remainder: the proof drives Next/Scan/Err/Close only,
// so the rest stay inert.
func (r *fakeInputInvalidRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *fakeInputInvalidRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *fakeInputInvalidRows) Values() ([]any, error) { return nil, nil }

func (r *fakeInputInvalidRows) RawValues() [][]byte { return nil }

func (r *fakeInputInvalidRows) Conn() *pgx.Conn { return nil }

// fakeInputInvalidPool binds one canned quarantine row behind the
// adminread Pool seam: the P-READ proof runs the real production
// store read path over this pool. The lane issues a single bounded
// SELECT (no fan-out), so every query reads the staged row.
type fakeInputInvalidPool struct {
	rows   *fakeInputInvalidRows
	err    error
	closed bool
}

func (p *fakeInputInvalidPool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.rows, nil
}

func (p *fakeInputInvalidPool) Close() { p.closed = true }

// fakeInputInvalidStore is the in-memory apilib InputInvalidStore the
// lane matrix tests run against: it records the filter and call count
// and answers canned rows or a canned error. It emits no spans by
// design: span evidence must come from the production reader store
// path (internal/adminread over a fake pool, see the P-READ proof
// below), never from a fake InputInvalidStore.
type fakeInputInvalidStore struct {
	items []apilibadmin.InputInvalidFact
	err   error
	got   apilibadmin.InputInvalidFactListFilter
	calls int
}

func (f *fakeInputInvalidStore) ListReducerInputInvalidFacts(_ context.Context, filter apilibadmin.InputInvalidFactListFilter) ([]apilibadmin.InputInvalidFact, error) {
	f.got = filter
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}
