// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Code-flow relay-leg termination spec posture (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#98, W6a). The four
// Code-flow POSTs mount natively in the W6a wave lane, but their
// Decision 5(c) fragment (go/internal/query/openapi/paths/code/flow.go)
// stays core-side for this lane: the fragment documents all four keys
// as one concatenated block whose entry-shaped port belongs to the
// follow-up spec-port lane, and a reshaped satellite subset would
// break the verbatim-port rule scripts/check-parity.sh gates. The
// routes are therefore disclosed spec absences (see
// contracts/code-flow-termination.md), the same posture the gap-04
// Compare lane records for its core-side fragment. A follow-up
// spec-port lane claims the keys when it ports the fragment
// entry-shaped (plus the CodeFlowRequest/CodeFlowResponse schemas);
// until then no served key duplicates a relayed route, and no
// mounted route promises what the document does not carry.

// TestSpec_CodeFlowStaysDisclosedAbsent pins the Code-flow spec
// posture: the served document carries none of the four flow keys
// while the fragment stays core-side. Lifting any absence (porting
// the fragment) belongs to the follow-up spec-port lane, not to the
// relay-leg termination.
func TestSpec_CodeFlowStaysDisclosedAbsent(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())

	for _, path := range []string{
		"/api/v0/code/flow/cfg-summary",
		"/api/v0/code/flow/pdg-summary",
		"/api/v0/code/flow/reaching-def",
		"/api/v0/code/flow/taint-path",
	} {
		if _, ok := served[path]; ok {
			t.Errorf("served spec carries %s, want it disclosed-absent until the follow-up spec-port lane", path)
		}
	}
}
