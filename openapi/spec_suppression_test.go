// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// TestSpec_ServesSuppressionFamily pins the native-spec contract for
// the suppression family (satellite-design Decision 5(c)): the assembled
// document is valid JSON and carries the single suppression upsert path
// with its pinned operationId. The route itself stays on its existing
// tier-C write-through dispatch — this lane ports the fragment only, no
// route moves. The lossless-assembly test pins that the served
// path equals the ported fragment, and scripts/check-parity.sh gates its
// line-verbatim provenance.
func TestSpec_ServesSuppressionFamily(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	item, ok := served["/api/v0/supply-chain/impact/suppressions"].(map[string]any)
	if !ok {
		t.Fatalf("paths lacks /api/v0/supply-chain/impact/suppressions: %v", keys(served))
	}
	op, _ := item["post"].(map[string]any)
	if got := op["operationId"]; got != "createVulnerabilitySuppression" {
		t.Errorf("suppressions post operationId = %v, want createVulnerabilitySuppression", got)
	}
	if got := op["summary"]; got != "Create or update an operator-authored vulnerability suppression" {
		t.Errorf("suppressions post summary = %v, want the pinned summary", got)
	}
}
