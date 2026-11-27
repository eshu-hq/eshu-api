// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package sessionstore wires eshu-api to PostgreSQL for the D8
// session/identity/token surface (Refs eshu-hq/eshu-api#447, part of
// the ADR-0002 P5 sessions-and-authentication phase).
//
// Scope honesty (binding): this package owns the store/wiring layer
// only. No HTTP route terminates on it in this issue — api/d8.go stays
// tier-C dispatch — and the password/MFA verification writes stay
// unwired until P5/P6. It implements the four frozen v0 admission
// method sets from contracts/upstream/session_admission.go as local
// eshu-api types and deliberately does NOT import
// github.com/eshu-hq/eshu: the check-parity no-core-import gate stays
// green because every value crossing this boundary is a hash or
// metadata field, never a core type.
//
// Three invariants hold across every file here:
//
//  1. Hash-only storage. Session hashes, CSRF hashes, token hashes,
//     and subject-ID hashes travel as query arguments only, never
//     interpolated into SQL text and never returned by metadata
//     lists. Revocation is a revoked_at UPDATE, never a DELETE.
//  2. Least privilege by role. The resolver is a WRITE — it refreshes
//     last_seen_at/idle_expires_at inside the resolve — so it runs on
//     the PRIMARY with the eshu_api_session_rw role. Metadata-only
//     lists may ride the read replica only when the caller opts in
//     with WithReplica, because the replica trails by replication
//     lag; read-your-writes flows stay on the primary.
//  3. Global pool arithmetic. PoolBudget pins per-replica ceilings so
//     that PerReplicaMaxConns x (MaxReplicas + SurgeReplicas) plus the
//     core-api budget and headroom fits server max_connections. A bare
//     per-pod number is rejected by Validate.
//
// Secrets enter only through secret references: LoadCredentialFromEnv
// reads env-mapped secret keys and fails closed on empty input, and
// Rotating swaps credentials under audit (actor, role, timestamp,
// version — never the value) without dropping in-flight readers.
// Statement timeouts are pinned server-side by the migration in
// migrations/002_session_least_privilege_roles.sql, which runs
// out-of-band: runtime roles hold no DDL. Tables come from
// migrations/001_session_tables.sql, ordered first so the 002
// GRANTs resolve on a fresh database.
//
// Observability is stdlib-only until the OTEL dependency lands:
// errors and resolve/deny outcomes log a structured slog line
// (successful resolves carry the tenant_id metadata value);
// successful mutations and success-path lists/reads stay silent.
// Stats counts deny
// reasons (unresolved, store-error, CSRF mismatch, stale) so an
// operator can tell "PG down" from "credential invalid" from
// dashboards alone.
package sessionstore
