// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// LoginStateRetention bounds how long consumed and expired
// login_states rows survive as replay evidence: the out-of-band
// purger deletes rows whose expires_at is at or past this far in the
// past. Seven days keeps a week of correlation evidence for incident
// review (the denied counter carries no per-hash detail) while
// bounding the unauthenticated-flood growth the 003 header warns
// about (Refs eshu-hq/eshu-api#462).
const LoginStateRetention = 7 * 24 * time.Hour

const (
	// DefaultLoginStatePurgeBatch sizes one purge DELETE: small
	// enough to bound WAL and bloat per statement, large enough
	// that an hourly run clears flood-scale backlogs.
	DefaultLoginStatePurgeBatch = 5000
	// MaxLoginStatePurgeBatch caps the per-batch DELETE so one
	// operator flag cannot turn the hourly job into a
	// table-wide lock storm.
	MaxLoginStatePurgeBatch = 50000
)

// LoginStatePurger deletes consumed and expired login_states rows
// past retention. It runs out-of-band only — a scheduled job with
// the purge-only role — never on a Store and never in the serving
// pods. Purged rows already deny (expired or consumed), absent rows
// deny identically (zero, false, nil), and plain-INSERT create
// cannot resurrect: the purge can never allow or resurrect a
// state.
type LoginStatePurger struct {
	db  DB
	log *slog.Logger
}

// NewLoginStatePurger builds a purger over the purge-role handle. A
// nil log falls back to slog.Default, mirroring NewRotating.
func NewLoginStatePurger(db DB, log *slog.Logger) *LoginStatePurger {
	if log == nil {
		log = slog.Default()
	}
	return &LoginStatePurger{db: db, log: log}
}

// Purge deletes login_states rows with expires_at at or before
// now-minus-retention, in batches of at most batchLimit rows. The
// single expires_at predicate already covers consumed-old and
// expired-unconsumed rows: consumed_at always precedes expires_at,
// so the cutoff can never match a live row. Batches run as separate
// statements with no long transaction; a mid-loop failure returns
// the rows deleted so far with the error, and an idempotent rerun
// finishes the work. Zero rows is a (0, nil) success.
func (p *LoginStatePurger) Purge(ctx context.Context, now time.Time, retention time.Duration, batchLimit int) (int64, error) {
	if now.IsZero() {
		return 0, errors.New("sessionstore: login-state purge requires a non-zero now")
	}
	if retention <= 0 {
		return 0, errors.New("sessionstore: login-state purge requires a positive retention")
	}
	if batchLimit <= 0 || batchLimit > MaxLoginStatePurgeBatch {
		return 0, errors.New("sessionstore: login-state purge requires a batch limit within cap")
	}
	cutoff := now.UTC().Add(-retention)
	const query = `DELETE FROM login_states
		WHERE state_hash IN (
			SELECT state_hash FROM login_states
			WHERE expires_at <= $1
			ORDER BY expires_at
			LIMIT $2
		)`
	var deleted int64
	for {
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		res, err := p.db.ExecContext(ctx, query, cutoff, batchLimit)
		if err != nil {
			return deleted, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return deleted, err
		}
		deleted += n
		if n < int64(batchLimit) {
			break
		}
	}
	p.log.Info("session login-state purge",
		"op", "loginstate-purge",
		"cutoff", cutoff,
		"deleted", deleted,
	)
	return deleted, nil
}
