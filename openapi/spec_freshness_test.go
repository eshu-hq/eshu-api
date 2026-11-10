// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Wave-1 Freshness reads spec proof (Refs eshu-hq/eshu#9669, D13
// tracker eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#72). The
// Wave-1 lane ports three whole-file byte-identical fragments
// (freshness changed-since, generations, service changed-since) into
// the served document. All three core fragments are standalone
// entries — not paths-closing entries shaped like the Compare
// fragment — so the lane ports them line-verbatim whole-file instead
// of recording a disclosed absence. Byte identity of every ported file
// rides scripts/check-parity.sh; these tests pin the served assembly
// around those bytes.
//
// New $refs the Wave-1 lane introduces beyond the existing components
// block: none — the three fragments reference only the shared
// responses and schemas the D6 lane already carries. The generic
// TestSpec_LocalRefsResolve covers them without edits.

// TestSpec_ServesFreshnessFamily pins the native-spec contract for
// the Wave-1 Freshness family: the assembled document is valid JSON
// and carries every served Wave-1 path key with its pinned
// operationId.
func TestSpec_ServesFreshnessFamily(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantOp := range map[string]string{
		"/api/v0/freshness/changed-since":          "summarizeChangedSince",
		"/api/v0/freshness/generations":            "listGenerationLifecycle",
		"/api/v0/freshness/services/changed-since": "summarizeServiceChangedSince",
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
		for method, op := range item {
			got := op.(map[string]any)["operationId"]
			if got != wantOp {
				t.Errorf("%s %s operationId = %v, want %q", path, method, got, wantOp)
			}
		}
	}

	// The lossless-assembly property for the Wave-1 keys: the served
	// path objects equal the ported fragments' parsed path objects, so
	// the only reviewed delta to the pinned core fragments is the
	// fragment files themselves (gated byte-identical by
	// scripts/check-parity.sh).
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	for _, path := range []string{
		"/api/v0/freshness/changed-since",
		"/api/v0/freshness/generations",
		"/api/v0/freshness/services/changed-since",
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
