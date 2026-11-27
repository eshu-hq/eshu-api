// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
	"time"
)

// fakeConnector wires database/sql to the scripted fakeConn without
// any driver dependency, so the SQLDB adapter paths run under test on
// stdlib alone.
type fakeConnector struct{ conn *fakeConn }

func (c *fakeConnector) Connect(_ context.Context) (driver.Conn, error) { return c.conn, nil }

func (c *fakeConnector) Driver() driver.Driver { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(_ string) (driver.Conn, error) {
	return nil, errors.New("sessionstore test: no named drivers")
}

// fakeConn scripts one query set and one exec result.
type fakeConn struct {
	querySet [][]driver.Value
	execRows int64
	execErr  error
}

func (c *fakeConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("sessionstore test: prepare unused")
}

func (c *fakeConn) Close() error { return nil }

func (c *fakeConn) Begin() (driver.Tx, error) { return nil, errors.New("sessionstore test: no tx") }

func (c *fakeConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return &fakeDriverRows{set: c.querySet}, nil
}

func (c *fakeConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.execErr != nil {
		return nil, c.execErr
	}
	return driver.RowsAffected(c.execRows), nil
}

type fakeDriverRows struct {
	set [][]driver.Value
	pos int
}

func (r *fakeDriverRows) Columns() []string { return []string{"a", "b"} }

func (r *fakeDriverRows) Close() error { return nil }

func (r *fakeDriverRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.set) {
		return io.EOF
	}
	copy(dest, r.set[r.pos])
	r.pos++
	return nil
}

func TestSQLAdapter_RowQueryAndExec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	conn := &fakeConn{
		querySet: [][]driver.Value{{"x", int64(1)}, {"y", int64(2)}},
		execRows: 3,
	}
	db := sql.OpenDB(&fakeConnector{conn: conn})
	defer db.Close()
	adapted := &SQLDB{DB: db}

	var first string
	var firstN int64
	if err := adapted.QueryRowContext(ctx, "SELECT a, b WHERE hash = $1", "h").Scan(&first, &firstN); err != nil {
		t.Fatalf("adapter query row: %v", err)
	}
	if first != "x" || firstN != 1 {
		t.Fatalf("adapter query row = (%q, %d), want (x, 1)", first, firstN)
	}
	rows, err := adapted.QueryContext(ctx, "SELECT a, b", nil)
	if err != nil {
		t.Fatalf("adapter query: %v", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var a string
		var b int64
		if err := rows.Scan(&a, &b); err != nil {
			t.Fatalf("adapter scan: %v", err)
		}
		count++
	}
	if count != 2 {
		t.Fatalf("adapter must surface both scripted rows, got %d", count)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("adapter rows err: %v", err)
	}
	res, err := adapted.ExecContext(ctx, "UPDATE t SET revoked_at = $1", now)
	if err != nil {
		t.Fatalf("adapter exec: %v", err)
	}
	n, err := res.RowsAffected()
	if err != nil || n != 3 {
		t.Fatalf("adapter rows affected = %d, %v", n, err)
	}
}

func TestSQLAdapter_ErrorsPropagate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	boom := errors.New("conn refused")
	conn := &fakeConn{execErr: boom}
	db := sql.OpenDB(&fakeConnector{conn: conn})
	defer db.Close()
	adapted := &SQLDB{DB: db}
	if _, err := adapted.ExecContext(ctx, "UPDATE t SET x = 1"); !errors.Is(err, boom) {
		t.Fatalf("adapter must propagate exec errors, got %v", err)
	}
}
