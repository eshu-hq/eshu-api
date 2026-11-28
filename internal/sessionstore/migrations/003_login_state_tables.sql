-- SPDX-License-Identifier: MIT
-- Copyright (c) 2025-2026 eshu-hq
--
-- 003: single-use login-state table for the OIDC/GitHub/SAML sign-in
-- flows.
--
-- Refs eshu-hq/eshu-api#450 (ADR-0002 P5 wiring). Run out-of-band
-- (advisory-locked one-shot job or migration harness), NEVER from the
-- serving pods: runtime roles hold no DDL, and the pods never carry
-- owner credentials. Idempotent: safe to re-apply (IF NOT EXISTS
-- throughout). Ordered BEFORE 004: the 004 GRANTs fail with
-- 'relation does not exist' on a fresh database without this table.
-- Every column below is referenced by exactly the queries in
-- loginstate.go; no wider surface.
--
-- Column contracts the queries rely on:
--   login_states: state_hash primary key, hash-only storage (the
--     plaintext state/nonce/RelayState stay with the caller; only
--     their hashes reach the store). provider_kind is CHECKed to the
--     three sign-in flows. secondary_hash carries the SAML
--     RelayState hash and stays '' for OIDC/GitHub; nonce_hash
--     carries the OIDC nonce hash and stays '' for GitHub (plain
--     OAuth2, no nonce) and SAML. return_to_path is the caller's
--     already-sanitized same-origin path; the store never validates
--     redirect policy. issued_at/expires_at default NOW() pairs only
--     as fallbacks: callers set both and the CHECK rejects a dead
--     window. consumed_at stays NULL until the single atomic consume
--     stamps it; expiry is enforced by the consume predicate, not by
--     deletion — consumed and expired rows are retained as replay
--     evidence until a future out-of-band owner job purges them.
--   No expires_at index: the primary-key row lock (not a scan) is
--     the consume contention domain, and no serving sweeper exists
--     to use one.

BEGIN;

CREATE TABLE IF NOT EXISTS login_states (
    state_hash         TEXT PRIMARY KEY,
    provider_kind      TEXT NOT NULL CHECK (provider_kind IN ('oidc', 'github', 'saml')),
    provider_config_id TEXT NOT NULL,
    tenant_id          TEXT NOT NULL DEFAULT '',
    workspace_id       TEXT NOT NULL DEFAULT '',
    secondary_hash     TEXT NOT NULL DEFAULT '',
    nonce_hash         TEXT NOT NULL DEFAULT '',
    return_to_path     TEXT NOT NULL DEFAULT '',
    issued_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at         TIMESTAMPTZ NOT NULL,
    consumed_at        TIMESTAMPTZ,
    CHECK (expires_at > issued_at)
);

COMMIT;
