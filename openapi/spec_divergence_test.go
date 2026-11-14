// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Code-divergence relay-leg termination spec posture (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#125). The three
// divergence POSTs mount natively in the divergence wave lane, but
// their Decision 5(c) fragments stay core-side for this lane: the
// findings, investigate, and report entries all live in
// go/internal/query/openapi/paths/code/divergence.go.
// scripts/check-parity.sh gates — so, like the gap-04 Compare, W6a
// Code-flow, W4 search, G-a Code-graph, and Wave-4 Drift relay-leg
// lanes, the routes are disclosed spec absences (see
// contracts/code-divergence-termination.md), not a port. A follow-up
// spec-port lane claims the three keys when it ports the entries
// entry-shaped with their referenced schemas; until then no served
// key duplicates a relayed route, and no mounted route promises what
// the document does not carry.

// TestSpec_CodeDivergenceStaysDisclosedAbsent pins the divergence
// spec posture: the served document carries none of the three
// divergence keys while the fragments stay core-side. Lifting any
// absence (porting the entries) belongs to the follow-up spec-port
// lane, not to the relay-leg termination.
func TestSpec_CodeDivergenceStaysDisclosedAbsent(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())

	for _, path := range []string{
		"/api/v0/code/divergence/findings",
		"/api/v0/code/divergence/investigate",
		"/api/v0/code/divergence/report",
	} {
		if _, ok := served[path]; ok {
			t.Errorf("served spec carries %s, want it disclosed-absent until the follow-up spec-port lane", path)
		}
	}
}
