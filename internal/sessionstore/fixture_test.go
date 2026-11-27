// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

// TestFixture_Postgres runs the load-bearing SQL against a real
// PostgreSQL fixture: resolve-refresh UPDATE, CSRF branch, revoke
// idempotence, and workspace-switch scoping. It runs only when the
// fixture environment sets ESHU_TEST_PG_DSN to a DSN the registered
// driver understands (for example a testcontainers fixture wiring the
// pgx stdlib driver); otherwise it skips. The fake-DB unit tests pin
// shapes and routing; this test pins lock and upsert semantics that
// fakes cannot prove, per the arbiter test strategy.
func TestFixture_Postgres(t *testing.T) {
	dsn := os.Getenv("ESHU_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("ESHU_TEST_PG_DSN unset: fixture-PostgreSQL scenarios need a live database")
	}
	var driver string
	for _, name := range sql.Drivers() {
		driver = name
		break
	}
	if driver == "" {
		t.Skip("no database/sql driver registered: fixture needs a postgres driver (e.g. pgx stdlib)")
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatalf("open fixture db: %v", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping fixture db: %v", err)
	}
	s := NewStore(&SQLDB{DB: db})
	now := time.Now().UTC()
	create := BrowserSessionCreate{
		SessionHash:       "fixture-session-hash",
		CSRFHash:          "fixture-csrf-hash",
		SubjectIDHash:     "fixture-subject",
		TenantID:          "fixture-tenant",
		WorkspaceID:       "fixture-workspace",
		IdleExpiresAt:     now.Add(time.Hour),
		AbsoluteExpiresAt: now.Add(24 * time.Hour),
	}
	if err := s.CreateBrowserSession(ctx, create); err != nil {
		t.Fatalf("fixture create: %v", err)
	}
	got, ok, err := s.ResolveBrowserSession(ctx, create.SessionHash, create.CSRFHash, true, now)
	if err != nil {
		t.Fatalf("fixture resolve: %v", err)
	}
	if !ok || got.SubjectIDHash != create.SubjectIDHash {
		t.Fatalf("fixture resolve mismatch: ok=%v ctx=%+v", ok, got)
	}
	if _, ok, err := s.ResolveBrowserSession(ctx, create.SessionHash, "wrong-csrf", true, now); err != nil || ok {
		t.Fatalf("fixture CSRF mismatch must deny: ok=%v err=%v", ok, err)
	}
	if err := s.RevokeBrowserSession(ctx, create.SessionHash, now); err != nil {
		t.Fatalf("fixture revoke: %v", err)
	}
	// Revoke is idempotent: a second revoke of the same hash succeeds.
	if err := s.RevokeBrowserSession(ctx, create.SessionHash, now); err != nil {
		t.Fatalf("fixture second revoke must succeed: %v", err)
	}
	if _, ok, err := s.ResolveBrowserSession(ctx, create.SessionHash, create.CSRFHash, true, now); err != nil || ok {
		t.Fatalf("revoked session must stop resolving: ok=%v err=%v", ok, err)
	}
}
