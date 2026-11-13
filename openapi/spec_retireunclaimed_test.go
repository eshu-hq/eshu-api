// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// TestSpec_ServesRetireUnclaimedFamily pins the native-spec contract
// for the retire-unclaimed writer family (satellite-design Decision
// 5(c)): the assembled document is valid JSON and carries the single
// never-claimed workflow retirement POST, pinned by summary (the
// fragment has no operationId, like the other writer fragments); the
// lossless-assembly test and check-parity.sh gate the rest.
func TestSpec_ServesRetireUnclaimedFamily(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	item, ok := served["/api/v0/admin/workflow-retire-unclaimed"].(map[string]any)
	if !ok {
		t.Fatalf("paths lacks /api/v0/admin/workflow-retire-unclaimed: %v", keys(served))
	}
	op, _ := item["post"].(map[string]any)
	if got := op["summary"]; got != "Retire unclaimed workflow work items" {
		t.Errorf("post summary = %v, want the pinned summary", got)
	}
	if _, hasOpID := op["operationId"]; hasOpID {
		t.Errorf("post carries operationId %v, want none (the pinned fragment has none)", op["operationId"])
	}
}
