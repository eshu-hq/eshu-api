// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Code-divergence full-surface spec proof (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#125). The three divergence POSTs mount natively in the divergence wave lane and their Decision 5(c) fragment (divergence.go) now rides the served document; the D13 row-7 full-surface lane claims the keys the relay-leg lane disclosed absent.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesCodeDivergenceWave(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/code/divergence/findings":    {"post": "findCodeDivergence"},
		"/api/v0/code/divergence/investigate": {"post": "investigateCodeDivergence"},
		"/api/v0/code/divergence/report":      {"post": "reportCodeDivergence"},
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
		"/api/v0/code/divergence/findings",
		"/api/v0/code/divergence/investigate",
		"/api/v0/code/divergence/report",
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
