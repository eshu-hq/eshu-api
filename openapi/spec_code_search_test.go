// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Code-search relay-leg termination spec posture (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#113). The two code
// search POSTs mount natively in the W4 wave lane, but their Decision
// 5(c) fragments (go/internal/query/openapi/paths/code/routes.go and
// go/internal/query/openapi/paths/code/symbols.go) stay core-side for
// this lane: each fragment carries the lane entry first-in-file inside
// a multi-key block, and the entries reference schemas
// (CodeSearchResponse, SymbolSearchResponse, RankCursorConflict) the
// served document does not carry yet — porting one entry without its
// referenced blocks, or re-shaping a satellite subset across two
// fragments, would break the verbatim-port rule
// scripts/check-parity.sh gates. The routes are therefore disclosed
// spec absences (see contracts/code-search-termination.md), the same
// posture the gap-04 Compare and W6a Code-flow lanes record for their
// core-side fragments. A follow-up spec-port lane claims the keys when
// it ports both entries entry-shaped with their referenced schemas;
// until then no served key duplicates a relayed route, and no mounted
// route promises what the document does not carry.

// TestSpec_CodeSearchStaysDisclosedAbsent pins the code search spec
// posture: the served document carries neither search key while the
// fragments stay core-side. Lifting either absence (porting the
// entries) belongs to the follow-up spec-port lane, not to the
// relay-leg termination.
func TestSpec_CodeSearchStaysDisclosedAbsent(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())

	for _, path := range []string{
		"/api/v0/code/search",
		"/api/v0/code/symbols/search",
	} {
		if _, ok := served[path]; ok {
			t.Errorf("served spec carries %s, want it disclosed-absent until the follow-up spec-port lane", path)
		}
	}
}
