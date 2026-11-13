// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Code-graph relay-leg termination spec posture (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#121). The four G-a
// graph POSTs mount natively in the code-graph wave lane, but their
// Decision 5(c) fragments stay core-side for this lane: the
// call-chain and compare entries live in
// go/internal/query/openapi/paths/code/routes.go, the metrics entry
// lives in go/internal/query/openapi/paths/code/symbols.go, and the
// routes/callers entry lives in
// go/internal/query/openapi/paths/code/route_to_caller.go — and the
// call-chain, compare, and metrics entries reference the EntityRef
// schema the served document does not carry yet. Porting an entry
// without its referenced blocks, or re-shaping a satellite subset
// across three fragments, would break the verbatim-port rule
// scripts/check-parity.sh gates — so, like the gap-04 Compare, W6a
// Code-flow, W4 search, W8a, and Wave-4 Drift relay-leg lanes, the
// routes are disclosed spec absences (see
// contracts/code-graph-termination.md), not a port. A follow-up
// spec-port lane claims the four keys when it ports the entries
// entry-shaped with their referenced schemas; until then no served
// key duplicates a relayed route, and no mounted route promises what
// the document does not carry.

// TestSpec_CodeGraphStaysDisclosedAbsent pins the graph spec posture:
// the served document carries none of the four G-a keys while the
// fragments stay core-side. Lifting any absence (porting the entries)
// belongs to the follow-up spec-port lane, not to the relay-leg
// termination.
func TestSpec_CodeGraphStaysDisclosedAbsent(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())

	for _, path := range []string{
		"/api/v0/code/call-chain",
		"/api/v0/code/call-chain/compare",
		"/api/v0/code/call-graph/metrics",
		"/api/v0/code/routes/callers",
	} {
		if _, ok := served[path]; ok {
			t.Errorf("served spec carries %s, want it disclosed-absent until the follow-up spec-port lane", path)
		}
	}
}
