// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"testing"
)

// TestCompextWaveFrozen pins the frozen Wave-1b termination target
// entry by entry (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#101): exactly the 2 component-extension routes
// behind the versioned pin, v0. A wave change moves the ownership
// note, the fragment reference, and this mirror together.
func TestCompextWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []compextRoute{
		{Method: "GET", Path: "/api/v0/component-extensions", Family: "other", Serve: compextServeGated, Fragment: "go/internal/query/openapi/paths/catalog/component_extensions.go"},
		{Method: "GET", Path: "/api/v0/component-extensions/{component_id}/diagnostics", Family: "other", Serve: compextServeGated, Fragment: "go/internal/query/openapi/paths/catalog/component_extensions.go"},
	}

	if len(compextWave) != len(want) {
		t.Fatalf("compextWave rows = %d, want %d: the wave is frozen, move it deliberately", len(compextWave), len(want))
	}
	for i, w := range want {
		if compextWave[i] != w {
			t.Errorf("compextWave[%d] = %+v, want %+v: move the wave, the ownership note, and the fragment together", i, compextWave[i], w)
		}
	}
}
