// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// D3 admin/identity/recovery reads spec proof (Refs eshu-hq/eshu#9669,
// part of #6707). The D3 lane ports two new fragments into the served
// document: the six admin query POSTs (status.AdminReads, a
// line-verbatim subset of the pinned core admin.go) and the
// invitations list GET (auth.AdminInvitations, a line-verbatim subset
// of the pinned core routes.go carrying only the GET — the create POST
// sharing its path key stays relay until D8). Byte identity of every
// ported line rides scripts/check-parity.sh; these tests pin the
// served assembly around those bytes.
//
// New $refs the D3 lane introduces beyond the existing components
// block: none. The admin query POSTs and the invitations GET reference
// only BadRequest, Forbidden, InternalError, and ServiceUnavailable,
// all landed by earlier lanes. The generic
// TestSpec_LocalRefsResolve covers them without edits.

// TestSpec_ServesD3AdminReadsFamily pins the native-spec contract for
// the D3 admin reads: the assembled document is valid JSON and carries
// the six query-POST path keys with their pinned summaries (the
// fragment has no operationIds, like the admin-recovery fragment) plus
// the invitations path key with its pinned GET operationId.
func TestSpec_ServesD3AdminReadsFamily(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantSummary := range map[string]string{
		"/api/v0/admin/work-items/query":                   "Query fact work items",
		"/api/v0/admin/decisions/query":                    "Query projection decisions",
		"/api/v0/admin/dead-letters/query":                 "Query dead-letter work items",
		"/api/v0/admin/input-invalid-facts/query":          "Query durable input_invalid quarantine facts",
		"/api/v0/admin/changed-since/poisoned-links/query": "Query poisoned and retrying changed-since links",
		"/api/v0/admin/replay-events/query":                "Query replay events",
	} {
		item, ok := served[path].(map[string]any)
		if !ok {
			t.Errorf("paths lacks %s: %v", path, keys(served))
			continue
		}
		op, _ := item["post"].(map[string]any)
		if got := op["summary"]; got != wantSummary {
			t.Errorf("%s post summary = %v, want %q", path, got, wantSummary)
		}
		if _, hasOpID := op["operationId"]; hasOpID {
			t.Errorf("%s post carries operationId %v, want none (the pinned fragment has none)", path, op["operationId"])
		}
	}

	// The lossless-assembly property for the D3 keys: the seven served
	// path objects equal the ported fragments' parsed path objects, so
	// the only reviewed delta to the pinned core fragments is the
	// fragment files themselves (gated byte-identical by
	// scripts/check-parity.sh).
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	for _, path := range []string{
		"/api/v0/admin/work-items/query",
		"/api/v0/admin/decisions/query",
		"/api/v0/admin/dead-letters/query",
		"/api/v0/admin/input-invalid-facts/query",
		"/api/v0/admin/changed-since/poisoned-links/query",
		"/api/v0/admin/replay-events/query",
		"/api/v0/auth/local/invitations",
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

	// The invitations key serves the landed GET plus the D8 create POST
	// sharing its key: D8 ports the relay-only create POST into the
	// same fragment, so the served entry documents both landed
	// methods (pinned in full by the D8 spec tests).
	item, ok := served["/api/v0/auth/local/invitations"].(map[string]any)
	if !ok {
		t.Fatalf("paths lacks /api/v0/auth/local/invitations: %v", keys(served))
	}
	get, _ := item["get"].(map[string]any)
	if got := get["operationId"]; got != "listAdminInvitations" {
		t.Errorf("invitations get operationId = %v, want listAdminInvitations", got)
	}
	post, _ := item["post"].(map[string]any)
	if got := post["operationId"]; got != "createLocalIdentityInvitation" {
		t.Errorf("invitations post operationId = %v, want createLocalIdentityInvitation (D8 merged POST)", got)
	}
}
