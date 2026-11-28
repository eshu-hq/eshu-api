// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq
//
// Command eshu-api-purge-login-states deletes consumed and expired
// login_states rows past retention (Refs eshu-hq/eshu-api#462). It
// runs out-of-band only — an hourly scheduled job with
// concurrencyPolicy: Forbid, authenticating as the purge-only
// eshu_api_session_purge role — never in the serving pods, which
// keep no DELETE grant and must never mount the purge DSN.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/eshu-hq/eshu-api/internal/sessionstore"
)

// purgeDriverEnvVar names the database/sql driver the purge job
// opens. It defaults to "pgx" (the pgx stdlib driver the deployer
// registers); this module pins no driver dependency, so an
// unregistered driver fails closed at open.
const purgeDriverEnvVar = "ESHU_SESSION_PG_DRIVER"

// openPurgeDB opens the purge database with the named driver. It is
// a variable so tests inject a stub without a live database.
var openPurgeDB = sql.Open

// run executes the purge: parse flags, load the purge-only
// credential, open and ping the database, and delete rows past
// retention in batches. Every failure exits non-zero; zero rows is a
// (0, nil) success. The DSN travels by environment only, never by
// flag, so it stays out of the process table.
func run(args []string, getenv func(string) string, openDB func(driverName, dsn string) (*sql.DB, error), log *slog.Logger) error {
	flags := flag.NewFlagSet("eshu-api-purge-login-states", flag.ContinueOnError)
	retention := flags.Duration("retention", sessionstore.LoginStateRetention, "delete login_states rows expired at least this long ago")
	batch := flags.Int("batch", sessionstore.DefaultLoginStatePurgeBatch, "maximum rows per purge DELETE")
	timeout := flags.Duration("timeout", 5*time.Minute, "overall deadline for the purge run")
	if err := flags.Parse(args); err != nil {
		return err
	}
	// Fail fast on nonsense flags before loading credentials or
	// dialing the database; Purge re-validates before deleting.
	if *retention <= 0 {
		return fmt.Errorf("eshu-api-purge-login-states: -retention must be positive")
	}
	if *batch <= 0 || *batch > sessionstore.MaxLoginStatePurgeBatch {
		return fmt.Errorf("eshu-api-purge-login-states: -batch must be within cap")
	}
	driver := getenv(purgeDriverEnvVar)
	if driver == "" {
		driver = "pgx"
	}
	cred, err := sessionstore.LoadCredentialFromEnv(
		sessionstore.RoleSessionPurge,
		"ESHU_SESSION_PURGE_DSN",
		"ESHU_SESSION_PURGE_DSN_VERSION",
	)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	db, err := openDB(driver, cred.DSN())
	if err != nil {
		return fmt.Errorf("eshu-api-purge-login-states: open database: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("eshu-api-purge-login-states: ping database: %w", err)
	}
	purger := sessionstore.NewLoginStatePurger(&sessionstore.SQLDB{DB: db}, log)
	deleted, err := purger.Purge(ctx, time.Now(), *retention, *batch)
	if err != nil {
		return fmt.Errorf("eshu-api-purge-login-states: purge: %w", err)
	}
	log.Info("eshu-api-purge-login-states done", "deleted", deleted)
	return nil
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(os.Args[1:], os.Getenv, openPurgeDB, log); err != nil {
		log.Error("eshu-api-purge-login-states failed", "error", err)
		os.Exit(1)
	}
}
