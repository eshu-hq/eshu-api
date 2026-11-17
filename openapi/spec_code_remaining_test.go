// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// Code scan/investigation/secrets/cypher/language/relationships full-surface spec proof (Refs eshu-hq/eshu#9669; contracts/code-scan-termination.md, code-investigation-termination.md, code-secrets-termination.md, code-cypher-termination.md, code-language-termination.md, code-relationships-termination.md). The twelve remaining code POSTs mount natively in their wave lanes and their Decision 5(c) entries now ride the served document with their referenced schemas; the D13 row-7 full-surface lane claims the keys the relay-leg lanes disclosed absent.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesCodeRemainingWaves(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/code/complexity":                   {"post": "getComplexity"},
		"/api/v0/code/quality/inspect":              {"post": "inspectCodeQuality"},
		"/api/v0/code/imports/investigate":          {"post": "investigateImportDependencies"},
		"/api/v0/code/bundles":                      {"post": "searchCodeBundles"},
		"/api/v0/code/structure/inventory":          {"post": "inspectCodeInventory"},
		"/api/v0/code/topics/investigate":           {"post": "investigateCodeTopic"},
		"/api/v0/code/security/secrets/investigate": {"post": "investigateHardcodedSecrets"},
		"/api/v0/code/cypher":                       {"post": "runReadOnlyCypher"},
		"/api/v0/code/visualize":                    {"post": "runReadOnlyCypherVisualization"},
		"/api/v0/code/language-query":               {"post": "queryLanguageEntities"},
		"/api/v0/code/relationships":                {"post": "getCodeRelationships"},
		"/api/v0/code/relationships/story":          {"post": "getCodeRelationshipStory"},
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
		"/api/v0/code/complexity",
		"/api/v0/code/quality/inspect",
		"/api/v0/code/imports/investigate",
		"/api/v0/code/bundles",
		"/api/v0/code/structure/inventory",
		"/api/v0/code/topics/investigate",
		"/api/v0/code/security/secrets/investigate",
		"/api/v0/code/cypher",
		"/api/v0/code/visualize",
		"/api/v0/code/language-query",
		"/api/v0/code/relationships",
		"/api/v0/code/relationships/story",
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
