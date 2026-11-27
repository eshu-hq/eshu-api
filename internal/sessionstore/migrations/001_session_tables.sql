-- SPDX-License-Identifier: MIT
-- Copyright (c) 2025-2026 eshu-hq
--
-- 001: session/identity/token tables for eshu-api.
--
-- Refs eshu-hq/eshu-api#447 (ADR-0002 P5 wiring). Run out-of-band
-- (advisory-locked one-shot job or migration harness), NEVER from the
-- serving pods: runtime roles hold no DDL, and the pods never carry
-- owner credentials. Idempotent: safe to re-apply (IF NOT EXISTS
-- throughout). Ordered BEFORE 002: the 002 GRANTs fail with
-- 'relation does not exist' on a fresh database without these
-- tables. Every column below is referenced by exactly the queries
-- in store.go; no wider surface.
--
-- Column contracts the queries rely on:
--   browser_sessions: hash primary key, hash-only storage (no
--     plaintext tokens); issued_at/last_seen_at default NOW() so
--     metadata lists never scan NULL into time.Time; all_scopes
--     defaults FALSE so the switch predicate stays atomic.
--   identity_api_tokens: hash-only token record; created_at defaults
--     NOW(); revoked_at/rotated_at stay NULL until set.
--   identity_subjects: profile facts only; mfa_verified_at stays
--     NULL until verified (the MFA read maps NULL to zero time).

BEGIN;

CREATE TABLE IF NOT EXISTS browser_sessions (
    session_hash         TEXT PRIMARY KEY,
    csrf_token_hash      TEXT NOT NULL,
    subject_id_hash      TEXT NOT NULL,
    tenant_id            TEXT NOT NULL,
    workspace_id         TEXT NOT NULL,
    all_scopes           BOOLEAN NOT NULL DEFAULT FALSE,
    issued_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    idle_expires_at      TIMESTAMPTZ NOT NULL,
    absolute_expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at           TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS browser_sessions_subject_idx
    ON browser_sessions (subject_id_hash);

CREATE TABLE IF NOT EXISTS identity_api_tokens (
    token_id        TEXT PRIMARY KEY,
    token_hash      TEXT NOT NULL,
    subject_id_hash TEXT NOT NULL,
    name            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,
    rotated_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS identity_api_tokens_subject_idx
    ON identity_api_tokens (subject_id_hash);

CREATE TABLE IF NOT EXISTS identity_subjects (
    subject_id_hash TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL,
    mfa_enrolled    BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_verified_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    disabled_at     TIMESTAMPTZ
);

COMMIT;
