// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Login-state provider kinds. The store persists exactly these three
// sign-in flows; anything else fails closed at validation.
const (
	// LoginStateProviderOIDC marks an OIDC authorization-code flow state.
	LoginStateProviderOIDC = "oidc"
	// LoginStateProviderGitHub marks a GitHub OAuth2 flow state (plain
	// OAuth2: no nonce is ever stored for this kind).
	LoginStateProviderGitHub = "github"
	// LoginStateProviderSAML marks a SAML AuthnRequest state: StateHash
	// carries the RequestID hash and SecondaryHash the RelayState hash.
	LoginStateProviderSAML = "saml"
)

// LoginStates is the eshu-api-local port for the single-use
// login-state surface behind the OIDC, GitHub, and SAML sign-in
// flows (Refs eshu-hq/eshu-api#450, part of the ADR-0002 P5
// sessions-and-authentication phase). Create persists the hash-only
// record; Consume atomically stamps it so a replay of a consumed
// state — or any expired, unknown, or mismatched state — is
// rejected. Handler wiring lives in a later lane, not here.
type LoginStates interface {
	CreateLoginState(ctx context.Context, record LoginStateCreate) error
	ConsumeLoginState(ctx context.Context, req LoginStateConsume, now time.Time) (LoginState, bool, error)
}

var _ LoginStates = (*Store)(nil)

// LoginStateCreate carries the hash-only record a new login state
// persists. The plaintext state, nonce, and RelayState stay with the
// caller; only their hashes reach the store, matching the
// BrowserSessionCreate hash-args convention. SecondaryHash carries
// the SAML RelayState hash and stays empty for OIDC/GitHub;
// NonceHash carries the OIDC nonce hash and stays empty for GitHub
// and SAML. ReturnToPath is the caller's already-sanitized
// same-origin path; the store never validates redirect policy. The
// caller sets IssuedAt/ExpiresAt: the store invents no TTL duration
// and rejects a non-positive window.
type LoginStateCreate struct {
	StateHash        string
	ProviderKind     string
	ProviderConfigID string
	TenantID         string
	WorkspaceID      string
	SecondaryHash    string
	NonceHash        string
	ReturnToPath     string
	IssuedAt         time.Time
	ExpiresAt        time.Time
}

// LoginStateConsume scopes one atomic consume: the state hash plus
// the provider kind and secondary hash it was created under. All
// three must match the stored row or the consume denies without
// saying which field missed.
type LoginStateConsume struct {
	StateHash     string
	ProviderKind  string
	SecondaryHash string
}

// LoginState is the stored flow context a successful consume
// returns: where the session must land and which nonce the ID token
// must repeat. It carries hashes and metadata only, never secret
// material.
type LoginState struct {
	ProviderConfigID string
	TenantID         string
	WorkspaceID      string
	NonceHash        string
	ReturnToPath     string
	ExpiresAt        time.Time
}

// validLoginStateKind reports whether kind names a sign-in flow the
// store persists.
func validLoginStateKind(kind string) bool {
	switch kind {
	case LoginStateProviderOIDC, LoginStateProviderGitHub, LoginStateProviderSAML:
		return true
	default:
		return false
	}
}

// CreateLoginState inserts the hash-only login-state record with a
// plain INSERT and no ON CONFLICT upsert: a state-hash collision
// fails loudly and must never resurrect a consumed or expired row.
// Validation rejects before touching the database: empty state hash
// or provider-config ID, an unknown provider kind, or a zero or
// non-positive issued-to-expiry window.
func (s *Store) CreateLoginState(ctx context.Context, record LoginStateCreate) error {
	if record.StateHash == "" {
		return fmt.Errorf("sessionstore: login state requires a state hash")
	}
	if !validLoginStateKind(record.ProviderKind) {
		return fmt.Errorf("sessionstore: unknown login-state provider kind %q", record.ProviderKind)
	}
	if record.ProviderConfigID == "" {
		return fmt.Errorf("sessionstore: login state requires a provider config ID")
	}
	if record.IssuedAt.IsZero() || record.ExpiresAt.IsZero() || !record.ExpiresAt.After(record.IssuedAt) {
		return errors.New("sessionstore: login state requires a positive issued-to-expiry window")
	}
	const query = `INSERT INTO login_states
		(state_hash, provider_kind, provider_config_id, tenant_id, workspace_id, secondary_hash, nonce_hash, return_to_path, issued_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	_, err := s.db.ExecContext(ctx, query,
		record.StateHash, record.ProviderKind, record.ProviderConfigID,
		record.TenantID, record.WorkspaceID, record.SecondaryHash,
		record.NonceHash, record.ReturnToPath,
		record.IssuedAt.UTC(), record.ExpiresAt.UTC(),
	)
	if err != nil {
		s.stats.StoreErrors.Add(1)
		s.log.Error("session store error", "op", "loginstate-create", "error", err)
	}
	return err
}

// ConsumeLoginState atomically consumes a live login state and
// returns its stored flow context. Exactly one statement stamps
// consumed_at under every deny predicate (unconsumed, unexpired,
// provider and secondary match), so parallel duplicate callbacks
// contend on the one primary-key row and PostgreSQL row-lock
// recheck leaves exactly one winner — the same mechanism as
// ResolveBrowserSession. Empty hashes deny without touching the
// database; unknown, replayed, expired, or mismatched states deny
// with ok=false and no error (no oracle distinguishes them); store
// failures propagate so the caller answers 503, never allow. The
// expiry boundary is strict: expires_at equal to now is dead.
func (s *Store) ConsumeLoginState(ctx context.Context, req LoginStateConsume, now time.Time) (LoginState, bool, error) {
	var zero LoginState
	if req.StateHash == "" {
		s.stats.LoginStateDenied.Add(1)
		return zero, false, nil
	}
	const query = `UPDATE login_states
		SET consumed_at = $4
		WHERE state_hash = $1 AND provider_kind = $2 AND secondary_hash = $3
			AND consumed_at IS NULL AND expires_at > $4
		RETURNING provider_config_id, tenant_id, workspace_id, nonce_hash, return_to_path, expires_at`
	var out LoginState
	err := s.db.QueryRowContext(ctx, query, req.StateHash, req.ProviderKind, req.SecondaryHash, now.UTC()).Scan(
		&out.ProviderConfigID, &out.TenantID, &out.WorkspaceID,
		&out.NonceHash, &out.ReturnToPath, &out.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		s.stats.LoginStateDenied.Add(1)
		return zero, false, nil
	}
	if err != nil {
		s.stats.StoreErrors.Add(1)
		s.log.Error("session store error", "op", "loginstate-consume", "error", err)
		return zero, false, err
	}
	s.stats.LoginStateConsumed.Add(1)
	return out, true, nil
}
