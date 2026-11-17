// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// W8b-narrow full-surface spec proof (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#101). The 15 narrow-family routes mount natively in the W8b-narrow wave lane and their Decision 5(c) fragments now ride the served document; the D13 row-7 full-surface lane claims the keys the satellite termination lane disclosed absent.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesW8bNarrowWave(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/kubernetes/correlations":             {"get": "listKubernetesCorrelations"},
		"/api/v0/observability/coverage/correlations": {"get": "listObservabilityCoverageCorrelations"},
		"/api/v0/work-items/evidence":                 {"get": "listWorkItemEvidence"},
		"/api/v0/semantic/code-hints":                 {"get": "listSemanticCodeHints"},
		"/api/v0/semantic/documentation-observations": {"get": "listSemanticDocumentationObservations"},
		"/api/v0/dependencies":                        {"get": "listDependencies"},
		"/api/v0/graph/entities":                      {"get": "browseGraphEntityInventory"},
		"/api/v0/images":                              {"get": "listContainerImages"},
		"/api/v0/images/tag-history":                  {"get": "listContainerImageTagHistory"},
		"/api/v0/incidents/{incident_id}/context":     {"get": "getIncidentContext"},
		"/api/v0/runtime-declarations/query":          {"post": "listRuntimeVersionDeclarations"},
		"/api/v0/cloud/inventory":                     {"get": "listCloudResourceInventory"},
		"/api/v0/cloud/ssm-host-inventory":            {"get": "listSSMHostInventory"},
		"/api/v0/service-catalog/correlations":        {"get": "listServiceCatalogCorrelations"},
		"/api/v0/codeowners/ownership":                {"get": "listCodeownersOwnership"},
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
		"/api/v0/kubernetes/correlations",
		"/api/v0/observability/coverage/correlations",
		"/api/v0/work-items/evidence",
		"/api/v0/semantic/code-hints",
		"/api/v0/semantic/documentation-observations",
		"/api/v0/dependencies",
		"/api/v0/graph/entities",
		"/api/v0/images",
		"/api/v0/images/tag-history",
		"/api/v0/incidents/{incident_id}/context",
		"/api/v0/runtime-declarations/query",
		"/api/v0/cloud/inventory",
		"/api/v0/cloud/ssm-host-inventory",
		"/api/v0/service-catalog/correlations",
		"/api/v0/codeowners/ownership",
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
