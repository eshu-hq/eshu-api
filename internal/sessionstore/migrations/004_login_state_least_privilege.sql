-- SPDX-License-Identifier: MIT
-- Copyright (c) 2025-2026 eshu-hq
--
-- 004: least-privilege grants for the single-use login-state table.
--
-- Refs eshu-hq/eshu-api#450 (ADR-0002 P5 wiring). Run out-of-band
-- (advisory-locked one-shot job or migration harness), NEVER from the
-- serving pods: runtime roles hold no DDL, and the pods never carry
-- owner credentials. Idempotent: safe to re-apply. Ordered AFTER
-- 003: every GRANT below names the table 003 creates.
--
-- Consume is a write (it stamps consumed_at) on read-your-writes
-- rows, so only the PRIMARY read-write role is granted: the
-- read-only replica role holds no grant on login_states. No DELETE
-- anywhere (consumed and expired rows are retained as replay
-- evidence); no TRUNCATE, no DDL.

BEGIN;

-- The 002 default-privilege revocation already closes future tables;
-- re-assert the table grant surface explicitly for this table.
GRANT SELECT, INSERT, UPDATE ON login_states TO eshu_api_session_rw;

COMMIT;
