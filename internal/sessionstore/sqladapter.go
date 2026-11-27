// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"database/sql"
)

// SQLRow adapts *sql.Row to Row.
type SQLRow struct{ row *sql.Row }

// Scan implements Row.
func (r *SQLRow) Scan(dest ...any) error { return r.row.Scan(dest...) }

// SQLRows adapts *sql.Rows to Rows.
type SQLRows struct{ rows *sql.Rows }

// Close implements Rows.
func (r *SQLRows) Close() error { return r.rows.Close() }

// Err implements Rows.
func (r *SQLRows) Err() error { return r.rows.Err() }

// Next implements Rows.
func (r *SQLRows) Next() bool { return r.rows.Next() }

// Scan implements Rows.
func (r *SQLRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }

// SQLResult adapts sql.Result to Result.
type SQLResult struct{ res sql.Result }

// RowsAffected implements Result.
func (r *SQLResult) RowsAffected() (int64, error) { return r.res.RowsAffected() }

// SQLDB adapts *sql.DB to DB with any registered postgres driver.
// The driver stays the deployer's choice (wired by the fixture or
// the chart); this package pins no driver dependency.
type SQLDB struct{ DB *sql.DB }

// QueryRowContext implements DB.
func (d *SQLDB) QueryRowContext(ctx context.Context, query string, args ...any) Row {
	return &SQLRow{row: d.DB.QueryRowContext(ctx, query, args...)}
}

// QueryContext implements DB.
func (d *SQLDB) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	rows, err := d.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &SQLRows{rows: rows}, nil
}

// ExecContext implements DB.
func (d *SQLDB) ExecContext(ctx context.Context, query string, args ...any) (Result, error) {
	res, err := d.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &SQLResult{res: res}, nil
}
