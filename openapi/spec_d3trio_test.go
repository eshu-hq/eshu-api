// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// D3 Gap 02 trio spec proof (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#70). The trio lane ports three fragments into the
// served document: the delta-union completeness GET and the reindex
// progress GET (whole-file byte-identical ports of the pinned core
// files at the Gap 02 merge) and the shared-projection tuning-report
// GET (a line-verbatim subset of the pinned core admin.go carrying
// only this lane's key). Byte identity of every ported line rides
// scripts/check-parity.sh; these tests pin the served assembly
// around those bytes.
//
// New $refs the trio lane introduces beyond the existing components
// block: none. The three GETs reference only inline descriptions and
// the shared BadRequest, InternalError, and ServiceUnavailable
// responses, all landed by earlier lanes. The generic
// TestSpec_LocalRefsResolve covers them without edits.

// TestSpec_ServesD3TrioFamily pins the native-spec contract for the
// Gap 02 trio: the assembled document is valid JSON and carries the
// three GET path keys with their pinned summaries (the fragments have
// no operationIds, like the admin query POSTs, so there is none to
// pin — the pinned core entries carry none).
func TestSpec_ServesD3TrioFamily(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantSummary := range map[string]string{
		"/api/v0/admin/delta-union/completeness":        "Check delta-union graph completeness",
		"/api/v0/admin/reindex/progress":                "Report reindex watermark progress per scope",
		"/api/v0/admin/shared-projection/tuning-report": "Get shared projection tuning guidance",
	} {
		item, ok := served[path].(map[string]any)
		if !ok {
			t.Errorf("paths lacks %s: %v", path, keys(served))
			continue
		}
		op, _ := item["get"].(map[string]any)
		if got := op["summary"]; got != wantSummary {
			t.Errorf("%s get summary = %v, want %q", path, got, wantSummary)
		}
		if _, hasOpID := op["operationId"]; hasOpID {
			t.Errorf("%s get carries operationId %v, want none (the pinned fragment has none)", path, op["operationId"])
		}
	}

	// The lossless-assembly property for the trio keys: the three
	// served path objects equal the ported fragments' parsed path
	// objects, so the only reviewed delta to the pinned core
	// fragments is the fragment files themselves (gated
	// byte-identical by scripts/check-parity.sh).
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	for _, path := range []string{
		"/api/v0/admin/delta-union/completeness",
		"/api/v0/admin/reindex/progress",
		"/api/v0/admin/shared-projection/tuning-report",
	} {
		want, ok := fragment[path]
		if !ok {
			t.Fatalf("fragment lacks %s", path)
		}
		got, ok := served[path].(map[string]any)
		if !ok {
			t.Fatalf("served spec lacks %s", path)
		}
		if !jsonEqual(got, want) {
			t.Errorf("served %s differs from the ported fragment", path)
		}
	}
}
