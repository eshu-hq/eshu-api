// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

// D3 admin-reads Gap 02 trio termination (Refs eshu-hq/eshu#9669,
// part of #6707, satellite eshu-hq/eshu-api#70). This file owns the
// frozen wave table for the three Admin GETs the D3 lane left on the
// generic relay for want of a versioned pin: delta-union
// completeness, reindex progress, and the static shared-projection
// tuning report. The core pins merged as eshu-hq/eshu#10412 (merge
// 4a587431fd): DeltaUnionCompletenessReadAdmission v0 (Check),
// ReindexProgressReadAdmission v0 (the 3-method alias), and
// SharedProjectionTuningReport v0 (the status/detail constants the
// handler serves). The behavior reuses the D3 engine in
// d3_terminate.go: the same shared-key wave gate, the same family
// authorizer, the same never-split trailing-slash rule, and the same
// contract record (contracts/d3-admin-reads.md). The pins are
// recorded in contracts/admin-trio-read-admission.md — this lane
// claims no D1 ReadStore coverage for the trio.
//
// Production binding (carried from the D3 wave): dest owns each
// route (caller gate, family authorizer, response mapping) and calls
// the versioned core read API as its port transport — bounded client,
// forwarded credential and negotiation headers, no retry, no
// table/queue handles dest-side. Authz stays atomic core-side:
// dest allows only the shared-key (AllScopes) identity it can
// construct and core re-judges every forwarded call as backstop.
// Termination claims stay provisional until the arbiter re-reviews
// the D0 evidence (blocker B1).
//
// Scope source: the three pinned admission files at the Gap 02
// merge — no guessing. Explicitly OUT (disclosed, not silent):
//
//   - the D1 ReadStore surface (work items, dead letters,
//     input-invalid facts, poisoned links, replay events, decisions,
//     evidence): this lane records no new coverage for it;
//   - the suppressions mutation row: untouched by this lane;
//   - the D7 reindex mutation POST: the ReindexProgress pin names it
//     as out of scope (mutation and progress view meet only at the
//     watermark rows, never through a shared pin).

// d3TrioRoute is one frozen Gap 02 wave entry: the exact cross-repo
// surface this lane terminates. Method+Path+Family mirror the pinned
// admissions; Fragment names the Decision 5(c) fragment carrying the
// route's spec entry.
type d3TrioRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// d3TrioWave is the frozen Gap 02 termination target: the 3 Admin
// GETs the core pins admit. TestD3TrioWaveFrozen pins it entry by
// entry; a wave change moves the spec, the skeleton table, and this
// mirror together.
var d3TrioWave = []d3TrioRoute{
	{Method: "GET", Path: "/api/v0/admin/delta-union/completeness", Family: "admin", Serve: d3ServeGated, Fragment: "paths/status/delta_union_completeness.go"},
	{Method: "GET", Path: "/api/v0/admin/reindex/progress", Family: "admin", Serve: d3ServeGated, Fragment: "paths/status/reindex_progress.go"},
	{Method: "GET", Path: "/api/v0/admin/shared-projection/tuning-report", Family: "admin", Serve: d3ServeGated, Fragment: "paths/status/admin.go"},
}
