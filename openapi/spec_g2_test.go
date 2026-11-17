// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// G2 full-surface spec proof (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#101 RE:G2V2). The 7 G2 reads mount natively in the G2 lane and their Decision 5(c) fragments (collector_extraction_readiness.go, fact_schema_version.go, surface_inventory.go, visualization_packets.go, manifest_dependencies.go) now ride the served document; the D13 row-7 full-surface lane claims the keys the lane disclosed absent.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesG2Wave(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/collector-extraction-readiness":          {"get": "listCollectorExtractionReadiness"},
		"/api/v0/collector-extraction-readiness/{family}": {"get": "getCollectorExtractionReadiness"},
		"/api/v0/fact-schema-versions":                    {"get": "listFactSchemaVersions"},
		"/api/v0/fact-schema-versions/{fact_kind}":        {"get": "getFactSchemaVersion"},
		"/api/v0/surface-inventory":                       {"get": "listSurfaceInventory"},
		"/api/v0/visualizations/derive":                   {"post": "deriveVisualizationPacket"},
		"/api/v0/repository/manifest-dependencies":        {"get": "listRepositoryManifestDependencies"},
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
		"/api/v0/collector-extraction-readiness",
		"/api/v0/collector-extraction-readiness/{family}",
		"/api/v0/fact-schema-versions",
		"/api/v0/fact-schema-versions/{fact_kind}",
		"/api/v0/surface-inventory",
		"/api/v0/visualizations/derive",
		"/api/v0/repository/manifest-dependencies",
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
