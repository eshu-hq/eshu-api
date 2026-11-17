// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Code-flow full-surface spec proof (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#98, W6a). The four Code-flow POSTs mount natively in the W6a wave lane and their Decision 5(c) fragment (flow.go) now rides the served document; the D13 row-7 full-surface lane claims the keys the relay-leg lane disclosed absent.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesCodeFlowWave(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/code/flow/taint-path":   {"post": "inspectCodeFlowTaintPath"},
		"/api/v0/code/flow/reaching-def": {"post": "inspectCodeFlowReachingDef"},
		"/api/v0/code/flow/cfg-summary":  {"post": "inspectCodeFlowCFGSummary"},
		"/api/v0/code/flow/pdg-summary":  {"post": "inspectCodeFlowPDGSummary"},
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
		"/api/v0/code/flow/taint-path",
		"/api/v0/code/flow/reaching-def",
		"/api/v0/code/flow/cfg-summary",
		"/api/v0/code/flow/pdg-summary",
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
