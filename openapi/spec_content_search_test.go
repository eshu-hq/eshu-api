// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// W4 Content search spec proof (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#111). The W4 lane ports a line-verbatim subset of
// the pinned core content fragment (the four search entries; the point
// entries already shipped with the Wave-2 lane). The core file carries
// the lane entries mid-list with separator commas — not a
// paths-closing entry shaped like the Compare fragment — so the subset
// concatenates without re-shaping and no disclosed absence is
// recorded. Byte identity of every ported line rides
// scripts/check-parity.sh; these tests pin the served assembly around
// those bytes.
//
// New $refs the W4 lane introduces beyond the existing components
// block: EntityContentSearchResponse (with RankWalk) and the
// RankCursorConflict response (ported line-verbatim from the pinned
// core components sources). The generic TestSpec_LocalRefsResolve
// covers them without edits.

// TestSpec_ServesContentSearchFamily pins the native-spec contract for
// the W4 Content search family: the assembled document is valid JSON
// and carries every served wave path key with its pinned operationId.
func TestSpec_ServesContentSearchFamily(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantOp := range map[string]string{
		"/api/v0/content/files/search":               "searchFiles",
		"/api/v0/content/entities/search":            "searchEntities",
		"/api/v0/content/dockerfile-installs/search": "searchDockerfileInstalls",
		"/api/v0/content/php-config/search":          "searchPHPConfiguration",
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
			if method != "post" {
				t.Errorf("%s serves method %q, want exactly post", path, method)
			}
		}
	}

	// The lossless-assembly property for the wave keys: the served
	// path objects equal the ported fragment's parsed path objects, so
	// the only reviewed delta to the pinned core fragment is the
	// fragment file itself (gated line-verbatim by
	// scripts/check-parity.sh).
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	for _, path := range []string{
		"/api/v0/content/files/search",
		"/api/v0/content/entities/search",
		"/api/v0/content/dockerfile-installs/search",
		"/api/v0/content/php-config/search",
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
