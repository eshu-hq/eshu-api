// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Drift relay-leg termination spec posture (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#117). The two drift routes mount
// natively in the drift wave lane, but their Decision 5(c) fragments
// stay core-side for this lane: the findings entry lives in
// go/internal/query/openapi/paths/cloud/runtime_drift.go and the
// packet entry lives in
// go/internal/query/openapi/paths/evidence/investigations.go, and
// porting either entry without its referenced blocks would break the
// verbatim-port rule scripts/check-parity.sh gates — so, like the
// gap-04 Compare, W6a Code-flow, W4 search, and W8a lanes, the routes
// are disclosed spec absences (see
// contracts/cloud-runtime-drift-termination.md), not a port. A
// follow-up spec-port lane claims the keys when it ports both entries
// entry-shaped with their referenced schemas; until then no served
// key duplicates a relayed route, and no mounted route promises what
// the document does not carry. The drift-packet absence moves here
// from TestSpec_D4DisclosedOmissions with this lane: it is no longer
// relay-only, it is disclosed-absent.

// TestSpec_DriftStaysDisclosedAbsent pins the drift spec posture: the
// served document carries neither drift key while the fragments stay
// core-side. Lifting either absence (porting the entries) belongs to
// the follow-up spec-port lane, not to the relay-leg termination.
func TestSpec_DriftStaysDisclosedAbsent(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())

	for _, path := range []string{
		"/api/v0/cloud/runtime-drift/findings",
		"/api/v0/investigations/drift/packet",
	} {
		if _, ok := served[path]; ok {
			t.Errorf("served spec carries %s, want it disclosed-absent until the follow-up spec-port lane", path)
		}
	}
}
