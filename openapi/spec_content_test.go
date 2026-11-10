// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Wave-2 Content point-reads spec proof (Refs eshu-hq/eshu#9669, D13
// tracker eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#72). The
// Wave-2 lane ports a line-verbatim subset of the pinned core content
// fragment (the three point-read entries; the search entries stay
// core-side with the search legs, disclosed in
// contracts/content-point-reads-wave.md). The core file carries the
// lane entries mid-list with separator commas — not a paths-closing
// entry shaped like the Compare fragment — so the subset concatenates
// without re-shaping and no disclosed absence is recorded. Byte
// identity of every ported line rides scripts/check-parity.sh; these
// tests pin the served assembly around those bytes.
//
// New $refs the Wave-2 lane introduces beyond the existing components
// block: FileContent and EntityContent (ported line-verbatim into
// openapi/components.json from the pinned core components source).
// The generic TestSpec_LocalRefsResolve covers them without edits.

// TestSpec_ServesContentPointReadsFamily pins the native-spec
// contract for the Wave-2 Content family: the assembled document is
// valid JSON and carries every served wave path key with its pinned
// operationId.
func TestSpec_ServesContentPointReadsFamily(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantOp := range map[string]string{
		"/api/v0/content/files/read":    "readFile",
		"/api/v0/content/files/lines":   "readFileLines",
		"/api/v0/content/entities/read": "readEntity",
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
		"/api/v0/content/files/read",
		"/api/v0/content/files/lines",
		"/api/v0/content/entities/read",
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
