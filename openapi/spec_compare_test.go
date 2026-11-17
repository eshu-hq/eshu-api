// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Compare full-surface spec proof (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#72, gap 04). The Compare POST mounts natively in its gap-04 wave lane and its Decision 5(c) fragment (compare.go, re-indented from the core paths-closing shape to the satellite mid-list shape) now rides the served document; the D13 row-7 full-surface lane claims the key the relay-leg lane disclosed absent.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesCompareWave(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/compare/environments": {"post": "compareEnvironments"},
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
		"/api/v0/compare/environments",
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
