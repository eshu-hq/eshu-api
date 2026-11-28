// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"
)

// TestFixture_LoginState runs the load-bearing login-state SQL against
// a real PostgreSQL fixture: parallel-consume single-winner,
// replay/expiry/boundary denial, cross-provider and secondary-hash
// scoping, and duplicate-create failure without resurrection. It runs
// only when the fixture environment sets ESHU_TEST_PG_DSN; otherwise
// it skips and single-use stays UNPROVEN by the fake-DB unit tests
// alone. The fake-DB tests pin shapes and routing; this test pins the
// row-lock atomicity that fakes cannot prove.
func TestFixture_LoginState(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping fixture db: %v", err)
	}
	applyMigrations(t, db)
	s := NewStore(&SQLDB{DB: db})
	now := time.Now().UTC()

	// Parallel duplicate callbacks contend on the one primary-key row:
	// exactly one consume wins, the rest deny without error.
	race := LoginStateCreate{
		StateHash:        "fixture-login-race",
		ProviderKind:     LoginStateProviderOIDC,
		ProviderConfigID: "fixture-provider",
		TenantID:         "fixture-tenant",
		WorkspaceID:      "fixture-workspace",
		NonceHash:        "fixture-nonce",
		ReturnToPath:     "/dashboard",
		IssuedAt:         now,
		ExpiresAt:        now.Add(10 * time.Minute),
	}
	if err := s.CreateLoginState(ctx, race); err != nil {
		t.Fatalf("fixture create: %v", err)
	}
	const racers = 8
	var wg sync.WaitGroup
	wins := make(chan bool, racers)
	errs := make(chan error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{
				StateHash:    race.StateHash,
				ProviderKind: race.ProviderKind,
			}, now)
			wins <- ok
			errs <- err
		}()
	}
	wg.Wait()
	close(wins)
	close(errs)
	var won int
	for ok := range wins {
		if ok {
			won++
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("parallel consume must deny without error, got %v", err)
		}
	}
	if won != 1 {
		t.Fatalf("parallel consume must leave exactly one winner, got %d", won)
	}

	// Replay of the consumed state is rejected.
	if _, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{StateHash: race.StateHash, ProviderKind: race.ProviderKind}, now); err != nil || ok {
		t.Fatalf("replay of a consumed state must deny: ok=%v err=%v", ok, err)
	}

	// A duplicate create after consume fails loudly and does not
	// resurrect the row.
	if err := s.CreateLoginState(ctx, race); err == nil {
		t.Fatal("duplicate create must fail, never resurrect a consumed row")
	}
	if _, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{StateHash: race.StateHash, ProviderKind: race.ProviderKind}, now); err != nil || ok {
		t.Fatalf("failed duplicate create must not resurrect the row: ok=%v err=%v", ok, err)
	}

	// Expired and at-boundary states deny.
	for name, rec := range map[string]LoginStateCreate{
		"expired": {
			StateHash: "fixture-login-expired", ProviderKind: LoginStateProviderGitHub,
			ProviderConfigID: "fixture-provider", IssuedAt: now.Add(-10 * time.Minute), ExpiresAt: now.Add(-time.Minute),
		},
		"at-boundary": {
			StateHash: "fixture-login-boundary", ProviderKind: LoginStateProviderGitHub,
			ProviderConfigID: "fixture-provider", IssuedAt: now.Add(-10 * time.Minute), ExpiresAt: now,
		},
	} {
		if err := s.CreateLoginState(ctx, rec); err != nil {
			t.Fatalf("fixture create %s: %v", name, err)
		}
		if _, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{StateHash: rec.StateHash, ProviderKind: rec.ProviderKind}, now); err != nil || ok {
			t.Fatalf("fixture %s must deny: ok=%v err=%v", name, ok, err)
		}
	}

	// Cross-provider and wrong-secondary consumes deny.
	saml := LoginStateCreate{
		StateHash: "fixture-login-saml", ProviderKind: LoginStateProviderSAML,
		ProviderConfigID: "fixture-provider", SecondaryHash: "fixture-relay",
		IssuedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	}
	if err := s.CreateLoginState(ctx, saml); err != nil {
		t.Fatalf("fixture create saml: %v", err)
	}
	if _, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{StateHash: saml.StateHash, ProviderKind: LoginStateProviderOIDC, SecondaryHash: saml.SecondaryHash}, now); err != nil || ok {
		t.Fatalf("cross-provider consume must deny: ok=%v err=%v", ok, err)
	}
	if _, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{StateHash: saml.StateHash, ProviderKind: saml.ProviderKind, SecondaryHash: "wrong-relay"}, now); err != nil || ok {
		t.Fatalf("wrong-secondary consume must deny: ok=%v err=%v", ok, err)
	}
	got, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{StateHash: saml.StateHash, ProviderKind: saml.ProviderKind, SecondaryHash: saml.SecondaryHash}, now)
	if err != nil || !ok {
		t.Fatalf("correct SAML consume must win: ok=%v err=%v", ok, err)
	}
	if got.ProviderConfigID != saml.ProviderConfigID {
		t.Fatalf("fixture consume mismatch: %+v", got)
	}
}

// TestFixture_LoginStatePurge proves the retention purge against a
// real PostgreSQL fixture: rows expired past LoginStateRetention
// (consumed or not, including expires_at exactly on the cutoff)
// are deleted, rows within retention survive with unchanged consume
// verdicts, and a rerun deletes nothing. It runs only when the
// fixture environment sets ESHU_TEST_PG_DSN; otherwise it skips.
func TestFixture_LoginStatePurge(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping fixture db: %v", err)
	}
	applyMigrations(t, db)
	s := NewStore(&SQLDB{DB: db})
	now := time.Now().UTC()
	cutoff := now.Add(-LoginStateRetention)

	seed := func(name, hash string, issuedAt, expiresAt time.Time) {
		t.Helper()
		if err := s.CreateLoginState(ctx, LoginStateCreate{
			StateHash: hash, ProviderKind: LoginStateProviderOIDC,
			ProviderConfigID: "fixture-provider", IssuedAt: issuedAt, ExpiresAt: expiresAt,
		}); err != nil {
			t.Fatalf("fixture create %s: %v", name, err)
		}
	}

	// Survivors: a live row, recent-expired rows inside retention.
	seed("live", "purge-fixture-live", now, now.Add(10*time.Minute))
	seed("recent-unconsumed", "purge-fixture-recent-open", now.Add(-2*time.Hour), now.Add(-time.Hour))
	seed("recent-consumed", "purge-fixture-recent-done", now.Add(-2*time.Hour), now.Add(-time.Hour))
	if _, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{StateHash: "purge-fixture-recent-done", ProviderKind: LoginStateProviderOIDC}, now.Add(-90*time.Minute)); err != nil || !ok {
		t.Fatalf("fixture consume recent must win: ok=%v err=%v", ok, err)
	}

	// Purged: rows expired 8 days ago (past the 7-day retention) plus
	// a row with expires_at exactly on the cutoff (<= purges it).
	seed("old-unconsumed", "purge-fixture-old-open", now.Add(-8*24*time.Hour).Add(-10*time.Minute), now.Add(-8*24*time.Hour))
	seed("old-consumed", "purge-fixture-old-done", cutoff.Add(-24*time.Hour).Add(-10*time.Minute), cutoff.Add(-24*time.Hour))
	if _, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{StateHash: "purge-fixture-old-done", ProviderKind: LoginStateProviderOIDC}, cutoff.Add(-24*time.Hour).Add(-time.Minute)); err != nil || !ok {
		t.Fatalf("fixture consume old must win before expiry: ok=%v err=%v", ok, err)
	}
	seed("at-cutoff", "purge-fixture-cutoff", cutoff.Add(-10*time.Minute), cutoff)

	purger := NewLoginStatePurger(&SQLDB{DB: db}, nil)
	deleted, err := purger.Purge(ctx, now, LoginStateRetention, 2)
	if err != nil {
		t.Fatalf("fixture purge: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("fixture purge deleted = %d, want 3 (two old rows plus the cutoff row)", deleted)
	}

	// Purged rows deny like absent rows.
	for _, hash := range []string{"purge-fixture-old-open", "purge-fixture-old-done", "purge-fixture-cutoff"} {
		if _, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{StateHash: hash, ProviderKind: LoginStateProviderOIDC}, now); err != nil || ok {
			t.Fatalf("purged row %s must deny: ok=%v err=%v", hash, ok, err)
		}
	}

	// Survivors keep their verdicts: live wins, recent denies.
	if _, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{StateHash: "purge-fixture-live", ProviderKind: LoginStateProviderOIDC}, now); err != nil || !ok {
		t.Fatalf("live survivor must still win: ok=%v err=%v", ok, err)
	}
	for _, hash := range []string{"purge-fixture-recent-open", "purge-fixture-recent-done"} {
		if _, ok, err := s.ConsumeLoginState(ctx, LoginStateConsume{StateHash: hash, ProviderKind: LoginStateProviderOIDC}, now); err != nil || ok {
			t.Fatalf("retained row %s must still deny: ok=%v err=%v", hash, ok, err)
		}
	}

	rerun, err := purger.Purge(ctx, now, LoginStateRetention, 2)
	if err != nil {
		t.Fatalf("fixture purge rerun: %v", err)
	}
	if rerun != 0 {
		t.Fatalf("fixture purge rerun deleted = %d, want 0 (idempotent)", rerun)
	}
}
