// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"
)

// Row scans a single result row.
type Row interface {
	Scan(dest ...any) error
}

// Rows iterates a multi-row result.
type Rows interface {
	Close() error
	Err() error
	Next() bool
	Scan(dest ...any) error
}

// Result reports a mutation outcome.
type Result interface {
	RowsAffected() (int64, error)
}

// DB is the minimal query surface Store needs. It is satisfied by
// *SQLDB (database/sql, any registered postgres driver) and by test
// fakes, so unit tests prove shapes and routing while the
// ESHU_TEST_PG_DSN fixture test proves lock and upsert semantics
// against real PostgreSQL.
type DB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) Row
	QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (Result, error)
}

// Stats counts store outcomes by deny reason so an operator can tell
// "PG down" (StoreErrors) from "credential invalid" (Unresolved) from
// "CSRF mismatch" (CSRFMismatches) on dashboards alone.
type Stats struct {
	Unresolved     atomic.Int64
	StoreErrors    atomic.Int64
	CSRFMismatches atomic.Int64
	StaleDenials   atomic.Int64
	Resolves       atomic.Int64
}

// Store is the PostgreSQL session/identity/token wiring behind the
// four frozen v0 admission pins. The zero-value replica routing is
// primary-for-everything; pass WithReplica to opt metadata-only
// lists into the read replica. Compile-time assertions pin the port
// surface below.
var (
	_ Resolver         = (*Store)(nil)
	_ BrowserSessions  = (*Store)(nil)
	_ IdentityProfiles = (*Store)(nil)
	_ APITokens        = (*Store)(nil)
)

// Store consults db (the PRIMARY, eshu_api_session_rw role) for every
// resolve and mutation. Resolve is a WRITE — it refreshes last_seen
// and idle expiry — so it must never run on a read-only role or the
// read replica, whose lag would also reopen revoked sessions.
type Store struct {
	db              DB
	replica         DB
	replicaForLists bool
	log             *slog.Logger
	stats           *Stats
}

// Option tunes a Store.
type Option func(*Store)

// WithReplica opts metadata-only lists (ListSessionsBySubject,
// ListAPITokensBySubject, GetLocalIdentityMFAStatus) into the read
// replica. The risk rides in the name: replica lag means a caller
// may miss a row created milliseconds ago, so read-your-writes flows
// keep the default. Passing nil or false keeps primary for all.
func WithReplica(ro DB, enabled bool) Option {
	return func(s *Store) {
		s.replica = ro
		s.replicaForLists = enabled && ro != nil
	}
}

// WithLogger replaces the discard logger.
func WithLogger(log *slog.Logger) Option {
	return func(s *Store) {
		if log != nil {
			s.log = log
		}
	}
}

// NewStore builds a Store over the primary handle. The primary must
// carry the eshu_api_session_rw role; a superuser or shared 'dsn'
// handle is rejected by deployment review, not by code, because the
// DSN value never reaches this package — only the secret reference
// name does.
func NewStore(db DB, opts ...Option) *Store {
	s := &Store{db: db, log: slog.Default(), stats: &Stats{}}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Stats returns the deny-reason counters.
func (s *Store) Stats() *Stats { return s.stats }

// listDB returns the handle metadata-only lists may use.
func (s *Store) listDB() DB {
	if s.replicaForLists {
		return s.replica
	}
	return s.db
}

// ResolveBrowserSession refreshes last_seen/idle expiry and returns
// the auth context for a live session. Empty hashes deny without
// touching the database; unknown, stale, or CSRF-mismatched hashes
// deny without error; store failures propagate so the caller answers
// 503, never allow. The refresh UPDATE carries every deny predicate
// (revocation, absolute and idle expiry, CSRF match), so a denial
// never mutates session state: last_seen_at/idle_expires_at move
// only on an allowed resolve. A missed UPDATE runs one existence
// probe to classify the deny for Stats (stale vs CSRF vs unknown)
// without changing the uniform deny shape.
func (s *Store) ResolveBrowserSession(ctx context.Context, sessionHash, csrfTokenHash string, requireCSRF bool, now time.Time) (AuthContext, bool, error) {
	var zero AuthContext
	if sessionHash == "" {
		s.stats.Unresolved.Add(1)
		return zero, false, nil
	}
	const query = `UPDATE browser_sessions
		SET last_seen_at = $2, idle_expires_at = $3
		WHERE session_hash = $1 AND revoked_at IS NULL
			AND absolute_expires_at > $2 AND idle_expires_at > $2
			AND (NOT $4 OR csrf_token_hash = $5)
		RETURNING subject_id_hash, tenant_id, workspace_id, all_scopes, csrf_token_hash, absolute_expires_at`
	var out AuthContext
	var csrfHash string
	err := s.db.QueryRowContext(ctx, query, sessionHash, now.UTC(), now.UTC().Add(idleWindow), requireCSRF, csrfTokenHash).Scan(
		&out.SubjectIDHash, &out.TenantID, &out.WorkspaceID, &out.AllScopes, &csrfHash, &out.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		s.classifyDeny(ctx, sessionHash, csrfTokenHash, requireCSRF, now.UTC(), "resolve")
		return zero, false, nil
	}
	if err != nil {
		s.stats.StoreErrors.Add(1)
		s.log.Error("session store error", "op", "resolve", "error", err)
		return zero, false, err
	}
	s.stats.Resolves.Add(1)
	s.log.Info("session resolved", "op", "resolve", "tenant_hash", hashLogLabel(out.TenantID))
	return out, true, nil
}

// classifyDeny attributes a missed-row deny without changing its
// uniform shape. The probe reads only hashes and expiry flags; it
// never allows. A revoked or expired row counts StaleDenials, a live
// row with a wrong CSRF counts CSRFMismatches, and anything else
// (unknown hash, or a probe failure that must not allow) counts
// Unresolved.
func (s *Store) classifyDeny(ctx context.Context, sessionHash, csrfTokenHash string, requireCSRF bool, now time.Time, op string) {
	const probe = `-- session-exists-probe
		SELECT csrf_token_hash, revoked_at IS NOT NULL AS revoked, idle_expires_at, absolute_expires_at
		FROM browser_sessions WHERE session_hash = $1`
	var csrfHash string
	var revoked bool
	var idleExpiresAt, absoluteExpiresAt time.Time
	if err := s.db.QueryRowContext(ctx, probe, sessionHash).Scan(&csrfHash, &revoked, &idleExpiresAt, &absoluteExpiresAt); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.stats.StoreErrors.Add(1)
		}
		s.stats.Unresolved.Add(1)
		s.log.Info("session unresolved", "op", op)
		return
	}
	switch {
	case revoked || !idleExpiresAt.After(now) || !absoluteExpiresAt.After(now):
		s.stats.StaleDenials.Add(1)
		s.log.Info("session stale", "op", op)
	case requireCSRF && csrfHash != csrfTokenHash:
		s.stats.CSRFMismatches.Add(1)
		s.log.Info("session CSRF mismatch", "op", op)
	default:
		s.stats.Unresolved.Add(1)
		s.log.Info("session unresolved", "op", op)
	}
}

// CreateBrowserSession inserts the hash-only session record, refreshing
// an existing row on hash collision (upsert keeps concurrent
// retry-safe creates from failing against the hash primary key).
// The upsert never clears revoked_at: an explicitly revoked session
// stays revoked even if its create is retried or replayed.
func (s *Store) CreateBrowserSession(ctx context.Context, record BrowserSessionCreate) error {
	const query = `INSERT INTO browser_sessions
		(session_hash, csrf_token_hash, subject_id_hash, tenant_id, workspace_id, idle_expires_at, absolute_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (session_hash) DO UPDATE SET
			csrf_token_hash = EXCLUDED.csrf_token_hash,
			idle_expires_at = EXCLUDED.idle_expires_at,
			absolute_expires_at = EXCLUDED.absolute_expires_at`
	_, err := s.db.ExecContext(ctx, query,
		record.SessionHash, record.CSRFHash, record.SubjectIDHash,
		record.TenantID, record.WorkspaceID,
		record.IdleExpiresAt.UTC(), record.AbsoluteExpiresAt.UTC(),
	)
	if err != nil {
		s.stats.StoreErrors.Add(1)
		s.log.Error("session store error", "op", "create", "tenant_hash", hashLogLabel(record.SubjectIDHash), "error", err)
	}
	return err
}

// RevokeBrowserSession stamps revoked_at. It is idempotent: revoking
// an unknown or already-revoked hash still succeeds, so retries and
// double-submits converge.
func (s *Store) RevokeBrowserSession(ctx context.Context, sessionHash string, now time.Time) error {
	const query = `UPDATE browser_sessions SET revoked_at = $2
		WHERE session_hash = $1 AND revoked_at IS NULL`
	_, err := s.db.ExecContext(ctx, query, sessionHash, now.UTC())
	if err != nil {
		s.stats.StoreErrors.Add(1)
		s.log.Error("session store error", "op", "revoke", "error", err)
	}
	return err
}

// SwitchBrowserSessionWorkspace moves an all-scopes session across
// tenant/workspace. Scoped sessions match no row and deny with
// ok=false; the all_scopes predicate lives in SQL so the check and
// the move stay atomic under concurrency.
func (s *Store) SwitchBrowserSessionWorkspace(ctx context.Context, sessionHash, tenantID, workspaceID string, now time.Time) (AuthContext, bool, error) {
	var zero AuthContext
	if sessionHash == "" {
		s.stats.Unresolved.Add(1)
		return zero, false, nil
	}
	const query = `UPDATE browser_sessions
		SET tenant_id = $2, workspace_id = $3, last_seen_at = $4
		WHERE session_hash = $1 AND revoked_at IS NULL AND all_scopes
			AND absolute_expires_at > $4 AND idle_expires_at > $4
		RETURNING subject_id_hash, tenant_id, workspace_id, all_scopes, absolute_expires_at`
	var out AuthContext
	err := s.db.QueryRowContext(ctx, query, sessionHash, tenantID, workspaceID, now.UTC()).Scan(
		&out.SubjectIDHash, &out.TenantID, &out.WorkspaceID, &out.AllScopes, &out.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		s.classifyDeny(ctx, sessionHash, "", false, now.UTC(), "switch")
		return zero, false, nil
	}
	if err != nil {
		s.stats.StoreErrors.Add(1)
		s.log.Error("session store error", "op", "switch", "error", err)
		return zero, false, err
	}
	return out, true, nil
}

// ListSessionsBySubject returns at most limit metadata-only rows:
// the session hash computes Current server-side and is never
// selected. Negative limit/offset clamp to zero (PostgreSQL rejects
// negative LIMIT/OFFSET). It rides the replica only when WithReplica
// opted in.
func (s *Store) ListSessionsBySubject(ctx context.Context, subjectIDHash string, asOf time.Time, sessionHash string, limit, offset int) ([]BrowserSessionListItem, error) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	const query = `SELECT tenant_id, workspace_id, issued_at, last_seen_at, idle_expires_at, absolute_expires_at,
		($4 <> '' AND session_hash = $4) AS current
		FROM browser_sessions
		WHERE subject_id_hash = $1 AND revoked_at IS NULL AND issued_at <= $5
		ORDER BY issued_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.listDB().QueryContext(ctx, query, subjectIDHash, limit, offset, sessionHash, asOf.UTC())
	if err != nil {
		s.stats.StoreErrors.Add(1)
		return nil, err
	}
	defer rows.Close()
	var items []BrowserSessionListItem
	for rows.Next() {
		var item BrowserSessionListItem
		if err := rows.Scan(&item.TenantID, &item.WorkspaceID, &item.IssuedAt,
			&item.LastSeenAt, &item.IdleExpiresAt, &item.AbsoluteExpiresAt, &item.Current); err != nil {
			s.stats.StoreErrors.Add(1)
			return nil, err
		}
		items = append(items, item)
	}
	// Defensive trim: the contract is at most limit rows even if a
	// driver ignores the LIMIT parameter.
	if len(items) > limit {
		items = items[:limit]
	}
	return items, rows.Err()
}

// CreateLocalIdentityAPIToken persists the hash-only token record. A
// token_id collision with the same hash is a retry-safe no-op
// success; a collision with a different hash fails loudly instead of
// silently keeping the old hash live.
func (s *Store) CreateLocalIdentityAPIToken(ctx context.Context, record APITokenCreate) error {
	const query = `INSERT INTO identity_api_tokens (token_id, token_hash, subject_id_hash, name, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (token_id) DO UPDATE SET token_hash = EXCLUDED.token_hash
		WHERE identity_api_tokens.token_hash = EXCLUDED.token_hash`
	res, err := s.db.ExecContext(ctx, query,
		record.TokenID, record.TokenHash, record.SubjectIDHash, record.Name, record.ExpiresAt.UTC())
	if err != nil {
		s.stats.StoreErrors.Add(1)
		s.log.Error("session store error", "op", "token-create", "error", err)
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		s.stats.StoreErrors.Add(1)
		s.log.Error("session store error", "op", "token-create", "error", err)
		return err
	}
	if affected == 0 {
		s.log.Error("session store error", "op", "token-create", "error", errTokenIDCollision)
		return errTokenIDCollision
	}
	return nil
}

// ListAPITokensBySubject returns at most limit metadata-only token
// rows: token_hash is never selected. Negative limit/offset clamp to
// zero. It rides the replica only when WithReplica opted in.
func (s *Store) ListAPITokensBySubject(ctx context.Context, subjectIDHash string, asOf time.Time, limit, offset int) ([]APITokenListItem, error) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	const query = `SELECT token_id, name, created_at, expires_at, revoked_at IS NOT NULL AS revoked
		FROM identity_api_tokens WHERE subject_id_hash = $1 AND created_at <= $2
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`
	rows, err := s.listDB().QueryContext(ctx, query, subjectIDHash, asOf.UTC(), limit, offset)
	if err != nil {
		s.stats.StoreErrors.Add(1)
		return nil, err
	}
	defer rows.Close()
	var items []APITokenListItem
	for rows.Next() {
		var item APITokenListItem
		if err := rows.Scan(&item.TokenID, &item.Name, &item.CreatedAt, &item.ExpiresAt, &item.Revoked); err != nil {
			s.stats.StoreErrors.Add(1)
			return nil, err
		}
		items = append(items, item)
	}
	// Defensive trim: the contract is at most limit rows even if a
	// driver ignores the LIMIT parameter.
	if len(items) > limit {
		items = items[:limit]
	}
	return items, rows.Err()
}

// RevokeLocalIdentityAPIToken stamps revocation under the ownership
// predicate. Like session revoke it is idempotent.
func (s *Store) RevokeLocalIdentityAPIToken(ctx context.Context, revocation APITokenRevoke) error {
	const query = `UPDATE identity_api_tokens SET revoked_at = NOW()
		WHERE token_id = $1 AND subject_id_hash = $2 AND revoked_at IS NULL`
	_, err := s.db.ExecContext(ctx, query, revocation.TokenID, revocation.SubjectIDHash)
	if err != nil {
		s.stats.StoreErrors.Add(1)
		s.log.Error("session store error", "op", "token-revoke", "error", err)
	}
	return err
}

// RotateLocalIdentityAPIToken swaps the token hash in one UPDATE
// scoped by token ownership, so a subject can never rotate another
// subject's token even under concurrency.
func (s *Store) RotateLocalIdentityAPIToken(ctx context.Context, rotation APITokenRotate) error {
	const query = `UPDATE identity_api_tokens SET token_hash = $3, rotated_at = NOW()
		WHERE token_id = $1 AND subject_id_hash = $2 AND revoked_at IS NULL`
	_, err := s.db.ExecContext(ctx, query, rotation.TokenID, rotation.SubjectIDHash, rotation.NewTokenHash)
	if err != nil {
		s.stats.StoreErrors.Add(1)
		s.log.Error("session store error", "op", "token-rotate", "error", err)
	}
	return err
}

// GetLocalIdentityMFAStatus reads enrollment facts only: no
// credential material crosses, so the read-only role may serve it
// from the replica when opted in. A NULL mfa_verified_at (never
// verified) maps to the zero time.
func (s *Store) GetLocalIdentityMFAStatus(ctx context.Context, subjectIDHash string, now time.Time) (MFAStatus, error) {
	var out MFAStatus
	const query = `SELECT mfa_enrolled, mfa_verified_at FROM identity_subjects
		WHERE subject_id_hash = $1 AND disabled_at IS NULL AND $2 >= created_at`
	var verifiedAt sql.NullTime
	err := s.listDB().QueryRowContext(ctx, query, subjectIDHash, now.UTC()).Scan(&out.Enrolled, &verifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return MFAStatus{}, nil
	}
	if err != nil {
		s.stats.StoreErrors.Add(1)
		return MFAStatus{}, err
	}
	out.VerifiedAt = verifiedAt.Time
	return out, nil
}

// ResolveLocalIdentityUserID maps a subject hash to its user ID for
// the profile read path. Unknown subjects resolve ok=false.
func (s *Store) ResolveLocalIdentityUserID(ctx context.Context, subjectIDHash string) (string, bool, error) {
	if subjectIDHash == "" {
		return "", false, nil
	}
	const query = `SELECT user_id FROM identity_subjects
		WHERE subject_id_hash = $1 AND disabled_at IS NULL`
	var userID string
	err := s.listDB().QueryRowContext(ctx, query, subjectIDHash).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		s.stats.StoreErrors.Add(1)
		return "", false, err
	}
	return userID, true, nil
}

// idleWindow bounds how far one resolve extends idle expiry: thirty
// minutes, matching the session-cookie idle contract.
const idleWindow = 30 * time.Minute

// errTokenIDCollision reports a token_id already persisted under a
// different hash: success must mean the record exists with the given
// hash, so the collision fails loudly instead of silently keeping
// the old hash live.
var errTokenIDCollision = errors.New("sessionstore: token_id already exists with a different hash")

// hashLogLabel truncates a hash for log lines so even a misrouted
// label leaks at most a prefix. Callers must never log raw secrets;
// hashes are safe but short labels keep lines tight.
func hashLogLabel(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}
