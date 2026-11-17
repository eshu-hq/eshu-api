// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Dead-code full-surface spec proof (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#124). The three dead-code POSTs mount natively in the dead-code wave lane and their Decision 5(c) fragments (scan.go, cross_repo.go, investigation.go) now ride the served document; the D13 row-7 full-surface lane claims the keys the relay-leg lane disclosed absent.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesDeadCodeWave(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/code/dead-code":             {"post": "findDeadCode"},
		"/api/v0/code/dead-code/cross-repo":  {"post": "findCrossRepoDeadCode"},
		"/api/v0/code/dead-code/investigate": {"post": "investigateDeadCode"},
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
		"/api/v0/code/dead-code",
		"/api/v0/code/dead-code/cross-repo",
		"/api/v0/code/dead-code/investigate",
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
