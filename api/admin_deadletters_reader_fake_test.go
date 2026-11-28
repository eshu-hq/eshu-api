// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// errFakeDeadLettersExhausted fails a Scan that arrives with no row
// staged or the wrong column count: a shape mismatch in the fake,
// never a backend value.
var errFakeDeadLettersExhausted = errors.New("fake dead-letters pool: no row staged for scan")

// fakeDeadLetterRows serves one canned dead-letter reader row in
// SELECT column order behind pgx.Rows: the P-READ proof drives the
// real adminread store read path (SQL, scan) without a live
// database.
type fakeDeadLetterRows struct {
	staged bool
	closed bool
	at     time.Time
}

func (r *fakeDeadLetterRows) Next() bool { return r.staged }

func (r *fakeDeadLetterRows) Scan(dest ...any) error {
	if !r.staged || len(dest) != 11 {
		return errFakeDeadLettersExhausted
	}
	r.staged = false
	failure := "build_failed"
	values := []any{
		"w1", "s1", "g1", "build", "d", "collector-1", 3,
		sql.NullString{String: failure, Valid: true},
		r.at, r.at,
		sql.NullTime{},
	}
	for i, value := range values {
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
			return errFakeDeadLettersExhausted
		}
	}
	return nil
}

func (r *fakeDeadLetterRows) Err() error { return nil }

func (r *fakeDeadLetterRows) Close() { r.closed = true }

// The pgx.Rows remainder: the proof drives Next/Scan/Err/Close only,
// so the rest stay inert.
func (r *fakeDeadLetterRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *fakeDeadLetterRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *fakeDeadLetterRows) Values() ([]any, error) { return nil, nil }

func (r *fakeDeadLetterRows) RawValues() [][]byte { return nil }

func (r *fakeDeadLetterRows) Conn() *pgx.Conn { return nil }

// fakeDeadLetterPool binds one canned row (or a canned query error)
// behind the adminread Pool seam: the P-READ proof runs the real
// production store over this pool.
type fakeDeadLetterPool struct {
	rows   *fakeDeadLetterRows
	err    error
	closed bool
}

func (p *fakeDeadLetterPool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.rows, nil
}

func (p *fakeDeadLetterPool) Close() { p.closed = true }

// fakeDeadLetterStore is the in-memory apilib DeadLetterStore the
// lane matrix tests run against: it records the filter and call count
// and answers canned rows or a canned error. It emits no spans by
// design: span evidence must come from the production reader store
// path (internal/adminread over a fake pool, see the P-READ proof
// below), never from a fake DeadLetterStore.
type fakeDeadLetterStore struct {
	items []apilibadmin.DeadLetterWorkItem
	err   error
	got   apilibadmin.DeadLetterListFilter
	calls int
}

func (f *fakeDeadLetterStore) ListDeadLetterWorkItems(_ context.Context, filter apilibadmin.DeadLetterListFilter) ([]apilibadmin.DeadLetterWorkItem, error) {
	f.got = filter
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}
