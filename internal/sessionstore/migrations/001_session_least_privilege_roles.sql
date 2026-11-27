-- SPDX-License-Identifier: MIT
-- Copyright (c) 2025-2026 eshu-hq
--
-- 001: least-privilege session/identity/token roles for eshu-api.
--
-- Refs eshu-hq/eshu-api#447 (ADR-0002 P5 wiring). Run out-of-band
-- (advisory-locked one-shot job or migration harness), NEVER from the
-- serving pods: runtime roles hold no DDL, and the pods never carry
-- owner credentials. Idempotent: safe to re-apply.
--
-- Pool arithmetic (ADR-0002, global not per-pod):
--   PerReplicaMaxConns x (MaxReplicas + SurgeReplicas)
--     + CoreAPIBudget + Headroom <= ServerMaxConnections
-- Validate the same formula independently for the primary line
-- (rw pool + core-api budget) and the replica line (ro pool).
-- Worked example at ops-qa scale (replicas=1, surge=1, no HPA):
--   10 x (1 + 1) + 40 + 10 = 70 <= 100.
--
-- Secret hygiene: pods mount per-role DSNs from secret references
-- (e.g. eshu_api_session_rw_dsn / eshu_api_session_ro_dsn keys);
-- never reuse the postgres/superuser or replication passwords, never
-- bake a value into this file.

BEGIN;

-- Least-privilege roles. LOGIN: the serving pools authenticate AS
-- these roles with per-role passwords granted out-of-band via the
-- secret store (never baked into this file). NOLOGIN would refuse
-- password authentication and fail the serving pods at startup.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'eshu_api_session_rw') THEN
        CREATE ROLE eshu_api_session_rw WITH LOGIN;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'eshu_api_session_ro') THEN
        CREATE ROLE eshu_api_session_ro WITH LOGIN;
    END IF;
END
$$;

-- Belt and braces: the session surface starts from no access, so a
-- wider default_privileges change elsewhere cannot leak in.
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM eshu_api_session_rw, eshu_api_session_ro;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM eshu_api_session_rw, eshu_api_session_ro;
REVOKE ALL ON SCHEMA public FROM eshu_api_session_rw, eshu_api_session_ro;
GRANT USAGE ON SCHEMA public TO eshu_api_session_rw, eshu_api_session_ro;

-- Read-write role (PRIMARY only): the exact surface the store uses.
-- SELECT/INSERT/UPDATE on sessions and tokens, SELECT on the identity
-- profile columns. No DELETE anywhere (revocation is a revoked_at
-- UPDATE); no TRUNCATE, no DDL, no sequence ownership.
GRANT SELECT, INSERT, UPDATE ON browser_sessions TO eshu_api_session_rw;
GRANT SELECT, INSERT, UPDATE ON identity_api_tokens TO eshu_api_session_rw;
GRANT SELECT ON identity_subjects TO eshu_api_session_rw;

-- Read-only role (replica-eligible metadata lists only): SELECT on the
-- three tables, nothing else. It must never serve ResolveBrowserSession
-- (a write) or read-your-writes flows; the Store defaults those to
-- the primary and only WithReplica opts lists into the replica.
GRANT SELECT ON browser_sessions TO eshu_api_session_ro;
GRANT SELECT ON identity_api_tokens TO eshu_api_session_ro;
GRANT SELECT ON identity_subjects TO eshu_api_session_ro;

-- Server-side guardrails per role: a runaway list cannot hold a
-- connection past the dispatch budget (main.go ReadTimeout 10s).
ALTER ROLE eshu_api_session_rw SET statement_timeout = '8s';
ALTER ROLE eshu_api_session_ro SET statement_timeout = '8s';

COMMIT;
