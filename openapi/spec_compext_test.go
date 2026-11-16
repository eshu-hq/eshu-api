// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Wave-1b relay-leg termination spec posture (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#101). The 2
// component-extension routes mount natively in the Wave-1b lane, but
// their Decision 5(c) fragment stays core-side for this lane: the
// entry lives in its owning core paths file (see
// api/compext_wave.go Fragment column), and porting the entry
// without its referenced blocks would break the verbatim-port rule
// scripts/check-parity.sh gates — so, like the W8b-narrow, W8a,
// drift, and G lanes, the routes are disclosed spec absences (see
// contracts/component-extension-termination.md), not a port. A
// follow-up spec-port lane claims the keys when it ports the entries
// entry-shaped with their referenced schemas; until then no served
// key duplicates a relayed route, and no mounted route promises what
// the document does not carry.

// TestSpec_CompextStaysDisclosedAbsent pins the Wave-1b spec
// posture: the served document carries neither component-extension
// key while the fragment stays core-side. Lifting an absence
// (porting the entry) belongs to the follow-up spec-port lane, not
// to the relay-leg termination.
func TestSpec_CompextStaysDisclosedAbsent(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())

	for _, path := range []string{
		"/api/v0/component-extensions",
		"/api/v0/component-extensions/{component_id}/diagnostics",
	} {
		if _, ok := served[path]; ok {
			t.Errorf("served spec carries %s, want it disclosed-absent until the follow-up spec-port lane", path)
		}
	}
}
