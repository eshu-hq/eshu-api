// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// D7 admin-four spec proof (Refs eshu-hq/eshu#9669, part of #6707).
// The D7 lane ports one line-verbatim subset (AdminD7: the backfill,
// dead-letter, reindex, and skip POSTs) into the served document.
// Byte identity of every ported line rides scripts/check-parity.sh;
// these tests pin the served assembly around those bytes.
//
// New $refs the D7 lane introduces beyond the existing components
// block: none — the four entries name only the versioned response
// $refs (BadRequest, Conflict, InternalError, ServiceUnavailable)
// every lane already carries. The generic TestSpec_LocalRefsResolve
// covers them without edits.

// TestSpec_ServesD7Families pins the native-spec contract for the D7
// admin-four family: the assembled document is valid JSON and carries
// the four fragment path keys with their pinned summaries (the
// fragment has no operationIds, like the admin-recovery fragment), and
// exactly the one POST method each fragment entry documents.
func TestSpec_ServesD7Families(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantSummary := range map[string]string{
		"/api/v0/admin/backfill":    "Request a backfill",
		"/api/v0/admin/dead-letter": "Dead-letter work items",
		"/api/v0/admin/reindex":     "Request a fleet-wide or per-repository reindex",
		"/api/v0/admin/skip":        "Skip repository work items",
	} {
		item, ok := served[path].(map[string]any)
		if !ok {
			t.Errorf("paths lacks %s: %v", path, keys(served))
			continue
		}
		if len(item) != 1 {
			t.Errorf("%s serves %d methods, want exactly the one the fragment documents", path, len(item))
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
}

// TestSpec_D7FamilySectionMatchesFragment pins lossless assembly for
// the D7 keys: the served path objects equal the ported fragment's
// parsed path objects, so the only reviewed delta to the pinned core
// fragment is the fragment file itself (gated line-verbatim by
// scripts/check-parity.sh).
func TestSpec_D7FamilySectionMatchesFragment(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)

	for _, path := range []string{
		"/api/v0/admin/backfill",
		"/api/v0/admin/dead-letter",
		"/api/v0/admin/reindex",
		"/api/v0/admin/skip",
	} {
		want, ok := fragment[path]
		if !ok {
			t.Fatalf("fragment lacks %s", path)
		}
		got, ok := served[path]
		if !ok {
			t.Fatalf("served spec lacks %s", path)
		}
		if !jsonEqual(got, want) {
			t.Errorf("served %s differs from the ported fragment", path)
		}
	}
}
