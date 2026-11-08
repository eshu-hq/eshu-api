// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

// D3 admin/identity/recovery reads termination (Refs eshu-hq/eshu#9669,
// part of #6707). This file owns the frozen wave table: the 16 routes
// across the 4 D1-pinned read families the satellite terminates after
// the D2 Leaf B wave. It mirrors the D1 pins
// (AdminReadAdmissionContractVersion, AdminIdentityReadAdmissionContractVersion,
// GovernanceAuditReadAdmissionContractVersion,
// TerminalFailureReadAdmissionContractVersion, all v0 at the satellite
// pin #50) entry by entry; a wave change moves the eshu spec, the
// skeleton table, and this mirror together. The behavior lives in
// d3_terminate.go; the proofs in d3_conformance_test.go and the
// per-family suites.
//
// Production binding (carried from the D0 shim proof via the D2 wave):
// dest owns each route (caller gate, family authorizer, response
// mapping) and calls the versioned core read API as its port
// transport — bounded client, forwarded credential and negotiation
// headers, no retry, no table/queue handles dest-side. Authz stays
// atomic core-side: dest allows only the shared-key (AllScopes)
// identity it can construct and core re-judges every forwarded call as
// backstop (tenant scope, permission catalog, governance mode).
// Termination claims stay provisional until the arbiter re-reviews the
// D0 evidence (blocker B1); the contract record is
// contracts/d3-admin-reads.md.
//
// Scope source: go/cmd/api/CUTOVER_OWNERSHIP.md at the D2 lane base
// plus the four D1 pins — no guessing. Excluded from THIS wave
// (disclosed, not silent):
//
//   - the 3 Gap 02 Admin GETs (delta-union completeness, reindex
//     progress, shared-projection tuning-report): pinned separately
//     by the Gap 02 admissions and terminated by the trio lane in
//     api/d3trio_wave.go, not by this 16-route wave;
//   - the families D2 already terminated (provider-config reads, admin
//     sign-in-policy).

// D3 satellite serve kind. Every D3 route is wave-gated: shared-key
// callers terminate, unresolvable credential classes relay explicitly,
// credential-less callers (and junk-cookie carriers) keep the standing
// dest 401. There is no public carve-out in this lane — every D3 read
// is an admin/audit read the core serves authed.
const d3ServeGated = "gated"

// d3Route is one frozen D3 wave entry: the exact cross-repo surface
// this lane terminates. Method+Path+Family mirror the CUTOVER rows and
// the D1 pin that admits them; Fragment names the Decision 5(c)
// fragment that carries the route's spec entry.
type d3Route struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// d3Wave is the frozen D3 termination target: 16 routes across the 4
// D1-pinned families (admin query POSTs, admin identity GETs,
// governance-audit GETs, terminal-failure query POSTs).
// TestD3WaveFrozen pins it entry by entry; a wave change moves the
// spec, the skeleton table, and this mirror together.
var d3Wave = []d3Route{
	{Method: "POST", Path: "/api/v0/admin/work-items/query", Family: "admin", Serve: d3ServeGated, Fragment: "paths/status/admin.go"},
	{Method: "POST", Path: "/api/v0/admin/decisions/query", Family: "admin", Serve: d3ServeGated, Fragment: "paths/status/admin.go"},
	{Method: "POST", Path: "/api/v0/admin/dead-letters/query", Family: "admin", Serve: d3ServeGated, Fragment: "paths/status/admin.go"},
	{Method: "POST", Path: "/api/v0/admin/input-invalid-facts/query", Family: "admin", Serve: d3ServeGated, Fragment: "paths/status/admin.go"},
	{Method: "POST", Path: "/api/v0/admin/changed-since/poisoned-links/query", Family: "admin", Serve: d3ServeGated, Fragment: "paths/status/admin.go"},
	{Method: "POST", Path: "/api/v0/admin/replay-events/query", Family: "admin", Serve: d3ServeGated, Fragment: "paths/status/admin.go"},
	{Method: "GET", Path: "/api/v0/auth/admin/api-tokens", Family: "admin_identity", Serve: d3ServeGated, Fragment: "paths/auth/admin_reads.go"},
	{Method: "GET", Path: "/api/v0/auth/admin/idp-group-mappings", Family: "admin_identity", Serve: d3ServeGated, Fragment: "paths/auth/admin_reads.go"},
	{Method: "GET", Path: "/api/v0/auth/admin/idp-providers", Family: "admin_identity", Serve: d3ServeGated, Fragment: "paths/auth/admin_reads.go"},
	{Method: "GET", Path: "/api/v0/auth/admin/role-assignments", Family: "admin_identity", Serve: d3ServeGated, Fragment: "paths/auth/admin_reads.go"},
	{Method: "GET", Path: "/api/v0/auth/admin/roles", Family: "admin_identity", Serve: d3ServeGated, Fragment: "paths/auth/admin_reads.go"},
	{Method: "GET", Path: "/api/v0/auth/local/invitations", Family: "admin_identity", Serve: d3ServeGated, Fragment: "paths/auth/routes.go"},
	{Method: "GET", Path: "/api/v0/auth/admin/audit/events", Family: "governance_audit", Serve: d3ServeGated, Fragment: "paths/auth/admin_reads.go"},
	{Method: "GET", Path: "/api/v0/auth/admin/audit/summary", Family: "governance_audit", Serve: d3ServeGated, Fragment: "paths/auth/admin_reads.go"},
	{Method: "POST", Path: "/api/v0/admin/collector-failures/acknowledgements/query", Family: "terminal_failure", Serve: d3ServeGated, Fragment: "paths/status/collector_terminal_failure_acknowledgement.go"},
	{Method: "POST", Path: "/api/v0/admin/collector-failures/query", Family: "terminal_failure", Serve: d3ServeGated, Fragment: "paths/status/collector_terminal_failure_acknowledgement.go"},
}
