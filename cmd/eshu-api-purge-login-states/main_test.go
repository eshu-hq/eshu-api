// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// stubPurgeDriver answers Ping and DELETE with zero rows affected,
// so the purge success path runs on stdlib alone with no postgres
// driver or live database.
type stubPurgeDriver struct{}

func (stubPurgeDriver) Open(_ string) (driver.Conn, error) { return &stubPurgeConn{}, nil }

type stubPurgeConn struct{}

func (c *stubPurgeConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("eshu-api-purge-login-states test: prepare unused")
}

func (c *stubPurgeConn) Close() error { return nil }

func (c *stubPurgeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("eshu-api-purge-login-states test: no tx")
}

func (c *stubPurgeConn) Ping(_ context.Context) error { return nil }

func (c *stubPurgeConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}

func purgeTestOpenDB(t *testing.T) func(string, string) (*sql.DB, error) {
	t.Helper()
	sql.Register(t.Name(), stubPurgeDriver{})
	return func(_, _ string) (*sql.DB, error) { return sql.Open(t.Name(), "") }
}

func TestPurgeRun_EmptyDSNFailsClosed(t *testing.T) {
	t.Setenv("ESHU_SESSION_PURGE_DSN", "")
	calls := 0
	openDB := func(_, _ string) (*sql.DB, error) { calls++; return nil, nil }
	err := run([]string{}, func(string) string { return "" }, openDB, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("empty purge DSN must fail closed")
	}
	if calls != 0 {
		t.Fatal("empty DSN must fail before opening the database")
	}
}

func TestPurgeRun_OpenFailureReturnsError(t *testing.T) {
	t.Setenv("ESHU_SESSION_PURGE_DSN", "postgres://purge:FAKE@db:5432/eshu")
	boom := errors.New("eshu-api-purge-login-states test: dial fails")
	openDB := func(_, _ string) (*sql.DB, error) { return nil, boom }
	err := run([]string{}, func(string) string { return "" }, openDB, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if !errors.Is(err, boom) {
		t.Fatalf("run error = %v, want the open failure", err)
	}
}

func TestPurgeRun_InvalidBatchFailsBeforeOpen(t *testing.T) {
	t.Setenv("ESHU_SESSION_PURGE_DSN", "postgres://purge:FAKE@db:5432/eshu")
	calls := 0
	openDB := func(_, _ string) (*sql.DB, error) {
		calls++
		return nil, errors.New("eshu-api-purge-login-states test: must not open")
	}
	err := run([]string{"-batch", "0"}, func(string) string { return "" }, openDB, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("zero batch must fail validation")
	}
	if calls != 0 {
		t.Fatal("invalid batch must fail before opening the database")
	}
}

func TestPurgeRun_SuccessDeletesNothing(t *testing.T) {
	// Deliberately fake: this value never authenticates anywhere; it
	// exists so the test can prove the success path redacts it.
	const dsn = "postgres://purge:FAKE-SECRET-FOR-TEST-ONLY@db:5432/eshu"
	t.Setenv("ESHU_SESSION_PURGE_DSN", dsn)
	var logs strings.Builder
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	err := run([]string{"-timeout", "30s"}, func(string) string { return "" }, purgeTestOpenDB(t), log)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if strings.Contains(logs.String(), dsn) || strings.Contains(logs.String(), "FAKE-SECRET-FOR-TEST-ONLY") {
		t.Fatalf("purge log must never carry the DSN value: %s", logs.String())
	}
}
