// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Compare relay-leg termination spec posture (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72, gap 04). The Compare POST mounts
// natively in its gap-04 wave lane, but its Decision 5(c) fragment
// stays core-side for this lane: core shapes
// go/internal/query/openapi/paths/status/compare.go as the
// paths-closing entry of its own assembled document (last in core
// assembly, trailing `},`), so a byte-identical satellite port
// cannot concatenate mid-list — and a re-shaped satellite subset
// would break the verbatim-port rule scripts/check-parity.sh gates.
// The route is therefore a disclosed spec absence (see
// contracts/compare-termination.md), the same posture the setup
// lane records for its core-side fragments. A follow-up spec-port
// lane claims the key when core re-shapes the fragment or the lane
// ports it entry-shaped; until then no served key duplicates a
// relayed route, and no mounted route promises what the document
// does not carry.

// TestSpec_CompareStaysDisclosedAbsent pins the Compare spec
// posture: the served document carries no compare key while the
// fragment stays core-side. Lifting this absence (porting the
// fragment) belongs to the follow-up spec-port lane, not to the
// relay-leg termination.
func TestSpec_CompareStaysDisclosedAbsent(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())

	if _, ok := served["/api/v0/compare/environments"]; ok {
		t.Errorf("served spec carries /api/v0/compare/environments, want it disclosed-absent until the follow-up spec-port lane")
	}
}
