-- SPDX-License-Identifier: MIT
-- Copyright (c) 2025-2026 eshu-hq
--
-- 005: retention index and purge-only role for the single-use
-- login-state table.
--
-- Refs eshu-hq/eshu-api#462 (ADR-0002 P5 wiring). Run out-of-band
-- (advisory-locked one-shot job or migration harness), NEVER from the
-- serving pods: runtime roles hold no DDL, and the pods never carry
-- owner credentials. Idempotent: safe to re-apply (IF NOT EXISTS
-- throughout). Ordered AFTER 004: every statement below names the
-- table 003 creates. Never edit 001-004 for this lane: 005 lands
-- last.
--
-- Retention contract: the out-of-band purge job deletes login_states
-- rows whose expires_at is at or past LoginStateRetention (7 days)
-- before now, in batches. The single expires_at predicate covers
-- consumed-old and expired-unconsumed rows alike (consumed_at always
-- precedes expires_at, so the cutoff can never match a live row),
-- and purged rows already deny exactly like absent rows.
--
-- The plain btree below serves the purge's range-plus-ORDER-BY-plus-
-- LIMIT subselect. It is deliberately NOT partial on consumed_at: a
-- partial predicate would miss the expired-unconsumed flood rows the
-- purge must clear. Operators with an already-huge table may pre-
-- create this index CONCURRENTLY; this statement stays safe to
-- re-apply afterwards.

BEGIN;

-- Least-privilege purge role. LOGIN: the scheduled purge job
-- authenticates AS this role with a password granted out-of-band via
-- the secret store (never baked into this file). SELECT supports the
-- batched PK subselect; DELETE is granted on login_states ONLY, no
-- other table, no TRUNCATE, no DDL. The serving rw/ro roles keep no
-- DELETE anywhere.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'eshu_api_session_purge') THEN
        CREATE ROLE eshu_api_session_purge WITH LOGIN;
    END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO eshu_api_session_purge;
GRANT SELECT, DELETE ON login_states TO eshu_api_session_purge;

-- Per-batch background budget: the hourly job recurs outside the
-- request path, so it gets a longer ceiling than the 8s serving
-- dispatch budget pinned by 002.
ALTER ROLE eshu_api_session_purge SET statement_timeout = '30s';

CREATE INDEX IF NOT EXISTS login_states_expires_at_idx ON login_states (expires_at);

COMMIT;
