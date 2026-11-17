// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// IaC full-surface spec proof (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#102, W3). The six IaC read legs register natively on their read-admission pins and their Decision 5(c) fragments (routes.go, resources.go, and the replatforming and drift files) now ride the served document; the D13 row-7 full-surface lane claims the keys.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesIacWaves(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/iac/dead":                             {"post": "findDeadIaC"},
		"/api/v0/iac/unmanaged-resources":              {"post": "findUnmanagedResources"},
		"/api/v0/iac/terraform-import-plan/candidates": {"post": "proposeTerraformImportPlanCandidates"},
		"/api/v0/iac/management-status":                {"post": "getIaCManagementStatus"},
		"/api/v0/iac/management-status/explain":        {"post": "explainIaCManagementStatus"},
		"/api/v0/iac/resources":                        {"get": "listIaCResources"},
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
		"/api/v0/iac/dead",
		"/api/v0/iac/unmanaged-resources",
		"/api/v0/iac/terraform-import-plan/candidates",
		"/api/v0/iac/management-status",
		"/api/v0/iac/management-status/explain",
		"/api/v0/iac/resources",
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
