// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Setup full-surface spec proof (Refs eshu-hq/eshu#9669, eshu-api#44 revisit, D13 row-7 full-surface). The four setup routes (the public first-run carve-out) and their Decision 5(c) fragment (setup.go) now ride the served document with the core truthful public-route markers intact; see api/setup_spec_posture_test.go for the per-entry marker rule.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesSetupWave(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/auth/setup-state": {"get": "getAuthSetupState"},
		"/api/v0/auth/setup/claim": {"post": "claimSetup"},
		"/api/v0/auth/setup/admin": {"post": "createSetupAdmin"},
		"/api/v0/auth/setup/mfa":   {"post": "completeSetupMFA"},
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
		"/api/v0/auth/setup-state",
		"/api/v0/auth/setup/claim",
		"/api/v0/auth/setup/admin",
		"/api/v0/auth/setup/mfa",
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
