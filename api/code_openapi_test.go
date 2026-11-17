// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Code-lane served-spec proof at the route level (Refs eshu-hq/eshu#9669,
// D13 row-7 full-surface; contracts/code-*-termination.md). The openapi
// package tests pin the assembled document (see openapi/spec_code_*_test.go
// and openapi/spec_code_remaining_test.go); these tests pin that the
// native openapi.json route serves it to keyed callers with every code
// lane key and its pinned operationId, with or without a core origin
// configured.

// codeFlowSpecKeys are the path keys the code-flow wave adds to the
// served document: one entry per served wave route.
var codeFlowSpecKeys = []string{
	"/api/v0/code/flow/taint-path",
	"/api/v0/code/flow/reaching-def",
	"/api/v0/code/flow/cfg-summary",
	"/api/v0/code/flow/pdg-summary",
}

// codeSearchSpecKeys are the path keys the code-search wave adds to the
// served document: one entry per served wave route.
var codeSearchSpecKeys = []string{
	"/api/v0/code/search",
	"/api/v0/code/symbols/search",
}

// codeGraphSpecKeys are the path keys the code-graph wave adds to the
// served document: one entry per served wave route.
var codeGraphSpecKeys = []string{
	"/api/v0/code/call-chain",
	"/api/v0/code/call-chain/compare",
	"/api/v0/code/call-graph/metrics",
	"/api/v0/code/routes/callers",
}

// codeDeadSpecKeys are the path keys the code-dead wave adds to the
// served document: one entry per served wave route.
var codeDeadSpecKeys = []string{
	"/api/v0/code/dead-code",
	"/api/v0/code/dead-code/cross-repo",
	"/api/v0/code/dead-code/investigate",
}

// codeDivergenceSpecKeys are the path keys the code-divergence wave adds to the
// served document: one entry per served wave route.
var codeDivergenceSpecKeys = []string{
	"/api/v0/code/divergence/findings",
	"/api/v0/code/divergence/investigate",
	"/api/v0/code/divergence/report",
}

// codeRelationshipsSpecKeys are the path keys the code-relationships wave adds to the
// served document: one entry per served wave route.
var codeRelationshipsSpecKeys = []string{
	"/api/v0/code/relationships",
	"/api/v0/code/relationships/story",
}

// codeScanSpecKeys are the path keys the code-scan wave adds to the
// served document: one entry per served wave route.
var codeScanSpecKeys = []string{
	"/api/v0/code/complexity",
	"/api/v0/code/quality/inspect",
	"/api/v0/code/imports/investigate",
	"/api/v0/code/bundles",
}

// codeInvestigationSpecKeys are the path keys the code-investigation wave adds to the
// served document: one entry per served wave route.
var codeInvestigationSpecKeys = []string{
	"/api/v0/code/structure/inventory",
	"/api/v0/code/topics/investigate",
}

// codeSecretsSpecKeys are the path keys the code-secrets wave adds to the
// served document: one entry per served wave route.
var codeSecretsSpecKeys = []string{
	"/api/v0/code/security/secrets/investigate",
}

// codeCypherSpecKeys are the path keys the code-cypher wave adds to the
// served document: one entry per served wave route.
var codeCypherSpecKeys = []string{
	"/api/v0/code/cypher",
	"/api/v0/code/visualize",
}

// codeLanguageSpecKeys are the path keys the code-language wave adds to the
// served document: one entry per served wave route.
var codeLanguageSpecKeys = []string{
	"/api/v0/code/language-query",
}

// TestCodeNative_ServesWaveSpecKeys pins that the native spec route
// serves every code lane key: authed callers get the dest-assembled
// document with every code path and its pinned operationId, with or
// without a core origin configured.
func TestCodeNative_ServesWaveSpecKeys(t *testing.T) {
	t.Parallel()

	wantOps := map[string]map[string]string{
		"/api/v0/code/flow/taint-path":              {"post": "inspectCodeFlowTaintPath"},
		"/api/v0/code/flow/reaching-def":            {"post": "inspectCodeFlowReachingDef"},
		"/api/v0/code/flow/cfg-summary":             {"post": "inspectCodeFlowCFGSummary"},
		"/api/v0/code/flow/pdg-summary":             {"post": "inspectCodeFlowPDGSummary"},
		"/api/v0/code/search":                       {"post": "searchCode"},
		"/api/v0/code/symbols/search":               {"post": "findSymbolDefinitions"},
		"/api/v0/code/call-chain":                   {"post": "getCodeCallChain"},
		"/api/v0/code/call-chain/compare":           {"post": "compareCodeCallPaths"},
		"/api/v0/code/call-graph/metrics":           {"post": "inspectCallGraphMetrics"},
		"/api/v0/code/routes/callers":               {"post": "traceRouteCallers"},
		"/api/v0/code/dead-code":                    {"post": "findDeadCode"},
		"/api/v0/code/dead-code/cross-repo":         {"post": "findCrossRepoDeadCode"},
		"/api/v0/code/dead-code/investigate":        {"post": "investigateDeadCode"},
		"/api/v0/code/divergence/findings":          {"post": "findCodeDivergence"},
		"/api/v0/code/divergence/investigate":       {"post": "investigateCodeDivergence"},
		"/api/v0/code/divergence/report":            {"post": "reportCodeDivergence"},
		"/api/v0/code/relationships":                {"post": "getCodeRelationships"},
		"/api/v0/code/relationships/story":          {"post": "getCodeRelationshipStory"},
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
	}

	for _, origin := range []string{"http://core.invalid", ""} {
		srv := NewServer(origin, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodGet, "/api/v0/openapi.json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("origin %q: status = %d, want 200", origin, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("origin %q: served spec is not valid JSON: %v", origin, err)
		}
		paths, _ := doc["paths"].(map[string]any)
		for _, keys := range [][]string{
			codeFlowSpecKeys,
			codeSearchSpecKeys,
			codeGraphSpecKeys,
			codeDeadSpecKeys,
			codeDivergenceSpecKeys,
			codeRelationshipsSpecKeys,
			codeScanSpecKeys,
			codeInvestigationSpecKeys,
			codeSecretsSpecKeys,
			codeCypherSpecKeys,
			codeLanguageSpecKeys,
		} {
			for _, path := range keys {
				if _, ok := paths[path]; !ok {
					t.Errorf("origin %q: served spec lacks code key %s", origin, path)
				}
			}
		}
		for path, wantOps := range wantOps {
			item, ok := paths[path].(map[string]any)
			if !ok {
				continue
			}
			for method, wantID := range wantOps {
				op, _ := item[method].(map[string]any)
				if got := op["operationId"]; got != wantID {
					t.Errorf("origin %q: %s %s operationId = %v, want %q", origin, path, method, got, wantID)
				}
			}
		}
	}
}
