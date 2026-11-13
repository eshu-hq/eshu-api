// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Divergence relay-leg termination spec posture (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#125). The three
// divergence POSTs mount natively in the divergence wave lane, but their
// Decision 5(c) fragment stays core-side for this lane: all three
// entries live in go/internal/query/openapi/paths/code/divergence.go.
// Porting the entries without their referenced blocks would break the
// verbatim-port rule scripts/check-parity.sh gates — so, like the
// gap-04 Compare, W6a Code-flow, W4 search, Wave-7 G-a, W8a, dead-code,
// and Wave-4 Drift relay-leg lanes, the routes are disclosed spec absences
// (see contracts/code-divergence-termination.md), not a port. A
// follow-up spec-port lane claims the three keys when it ports the
// entries entry-shaped with their referenced schemas; until then no
// served key duplicates a relayed route, and no mounted route promises
// what the document does not carry.

// TestSpec_DivergenceStaysDisclosedAbsent pins the divergence spec
// posture: the served document carries none of the three divergence
// keys while the fragment stays core-side. Lifting any absence
// (porting the entries) belongs to the follow-up spec-port lane, not
// to the relay-leg termination.
func TestSpec_DivergenceStaysDisclosedAbsent(t *testing.T) {
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
