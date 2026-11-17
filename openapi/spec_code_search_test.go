// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Code-search full-surface spec proof (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#113, W4). The two code search POSTs mount natively in the W4 wave lane and their Decision 5(c) fragments (code routes.go and symbols.go entries) now ride the served document with their referenced schemas; the D13 row-7 full-surface lane claims the keys the relay-leg lane disclosed absent.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesCodeSearchWave(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/code/search":         {"post": "searchCode"},
		"/api/v0/code/symbols/search": {"post": "findSymbolDefinitions"},
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
		"/api/v0/code/search",
		"/api/v0/code/symbols/search",
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
