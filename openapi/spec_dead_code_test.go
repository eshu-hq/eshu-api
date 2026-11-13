// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Dead-code relay-leg termination spec posture (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#124). The three
// dead-code POSTs mount natively in the dead-code wave lane, but their
// Decision 5(c) fragments stay core-side for this lane: the scan entry
// lives in go/internal/query/openapi/paths/code/dead/scan.go, the
// cross-repo entry lives in
// go/internal/query/openapi/paths/code/dead/cross_repo.go, and the
// investigate entry lives in
// go/internal/query/openapi/paths/code/dead/investigation.go.
// Porting an entry without its referenced blocks would break the
// verbatim-port rule scripts/check-parity.sh gates — so, like the
// gap-04 Compare, W6a Code-flow, W4 search, Wave-7 G-a, W8a, and
// Wave-4 Drift relay-leg lanes, the routes are disclosed spec absences
// (see contracts/code-dead-code-termination.md), not a port. A
// follow-up spec-port lane claims the three keys when it ports the
// entries entry-shaped with their referenced schemas; until then no
// served key duplicates a relayed route, and no mounted route promises
// what the document does not carry.

// TestSpec_DeadCodeStaysDisclosedAbsent pins the dead-code spec
// posture: the served document carries none of the three dead-code
// keys while the fragments stay core-side. Lifting any absence
// (porting the entries) belongs to the follow-up spec-port lane, not
// to the relay-leg termination.
func TestSpec_DeadCodeStaysDisclosedAbsent(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())

	for _, path := range []string{
		"/api/v0/code/dead-code",
		"/api/v0/code/dead-code/cross-repo",
		"/api/v0/code/dead-code/investigate",
	} {
		if _, ok := served[path]; ok {
			t.Errorf("served spec carries %s, want it disclosed-absent until the follow-up spec-port lane", path)
		}
	}
}
