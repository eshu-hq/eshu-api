// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Code-graph full-surface spec proof (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#121, G-a). The four G-a graph POSTs mount natively in the code-graph wave lane and their Decision 5(c) fragments (routes.go, symbols.go, and route_to_caller.go entries, with the dangling core 501 carried inline) now ride the served document with their referenced schemas; the D13 row-7 full-surface lane claims the keys the relay-leg lane disclosed absent.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesCodeGraphWave(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/code/call-chain":         {"post": "getCodeCallChain"},
		"/api/v0/code/call-chain/compare": {"post": "compareCodeCallPaths"},
		"/api/v0/code/call-graph/metrics": {"post": "inspectCallGraphMetrics"},
		"/api/v0/code/routes/callers":     {"post": "traceRouteCallers"},
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
		"/api/v0/code/call-chain",
		"/api/v0/code/call-chain/compare",
		"/api/v0/code/call-graph/metrics",
		"/api/v0/code/routes/callers",
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
