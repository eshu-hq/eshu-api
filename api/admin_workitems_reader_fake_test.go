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
)

// errFakeWorkItemsExhausted fails a Scan that arrives with no row
// staged or the wrong column count: a shape mismatch in the fake,
// never a backend value.
var errFakeWorkItemsExhausted = errors.New("fake work-items pool: no row staged for scan")

// fakeWorkItemRows serves one canned reader row in SELECT column order
// behind pgx.Rows: the P-READ proof drives the real adminread store
// read path (SQL, scan, fold) without a live database.
type fakeWorkItemRows struct {
	staged bool
	closed bool
	at     time.Time
}

func (r *fakeWorkItemRows) Next() bool { return r.staged }

func (r *fakeWorkItemRows) Scan(dest ...any) error {
	if !r.staged || len(dest) != 15 {
		return errFakeWorkItemsExhausted
	}
	r.staged = false
	values := []any{
		"w1", "s1", "g1", "build", "d", "pending", 3, 0,
		sql.NullString{},
		sql.NullString{},
		sql.NullString{},
		r.at, r.at,
		sql.NullTime{},
		sql.NullString{},
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
			return errFakeWorkItemsExhausted
		}
	}
	return nil
}

func (r *fakeWorkItemRows) Err() error { return nil }

func (r *fakeWorkItemRows) Close() { r.closed = true }

// The pgx.Rows remainder: the proof drives Next/Scan/Err/Close only,
// so the rest stay inert.
func (r *fakeWorkItemRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *fakeWorkItemRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *fakeWorkItemRows) Values() ([]any, error) { return nil, nil }

func (r *fakeWorkItemRows) RawValues() [][]byte { return nil }

func (r *fakeWorkItemRows) Conn() *pgx.Conn { return nil }

// fakeWorkItemsPool binds one canned row (or a canned query error)
// behind the adminread Pool seam: the P-READ proof runs the real
// production store over this pool.
type fakeWorkItemsPool struct {
	rows   *fakeWorkItemRows
	err    error
	closed bool
}

func (p *fakeWorkItemsPool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.rows, nil
}

func (p *fakeWorkItemsPool) Close() { p.closed = true }
