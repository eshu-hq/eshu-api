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

// D4 Native-next served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, part of #6707). The openapi package tests pin the
// assembled document; these tests pin that the native openapi.json
// route serves it to keyed callers with the wave path keys present —
// for both a configured and an unconfigured core origin — while the
// key gate still fronts the route (the standing no-new-public-surface
// decision for the assembled spec).

// d4WaveSpecKeys are the path keys the D4 wave adds to the served
// document: one entry per served wave route. The evidence-packet key
// carries the deployable-unit GET only — the supply-chain, drift, and
// services packets sharing that file stay relay until their lanes.
// The repositories story key is absent by disclosure: its entry
// interpolates the shared schema.EvidenceBoundaries helper, which has
// no line-verbatim port without a core import (forbidden), so the
// served document omits that key until the helper ports as a component.
// The story route terminates natively regardless; only its served spec
// entry waits.
var d4WaveSpecKeys = []string{
	"/api/v0/catalog",
	"/api/v0/ci-cd/run-correlations",
	"/api/v0/ci-cd/run-correlations/count",
	"/api/v0/ci-cd/run-correlations/inventory",
	"/api/v0/cloud/resources",
	"/api/v0/documentation/evidence-packets/{packet_id}/freshness",
	"/api/v0/documentation/facts",
	"/api/v0/documentation/findings",
	"/api/v0/documentation/findings/count",
	"/api/v0/documentation/findings/inventory",
	"/api/v0/documentation/findings/{finding_id}/evidence-packet",
	"/api/v0/ecosystem/overview",
	"/api/v0/ecosystem/graph-summary",
	"/api/v0/evidence/admission-decisions",
	"/api/v0/evidence/bundle",
	"/api/v0/evidence/citations",
	"/api/v0/evidence/relationships/{resolved_id}",
	"/api/v0/infra/relationships",
	"/api/v0/infra/resources/count",
	"/api/v0/infra/resources/inventory",
	"/api/v0/infra/resources/search",
	"/api/v0/investigation-workflows",
	"/api/v0/investigation-workflows/resolve",
	"/api/v0/investigations/deployable-unit/packet",
	"/api/v0/package-registry/correlations",
	"/api/v0/package-registry/dependencies",
	"/api/v0/package-registry/dependency-chains",
	"/api/v0/package-registry/packages",
	"/api/v0/package-registry/packages/count",
	"/api/v0/package-registry/packages/inventory",
	"/api/v0/package-registry/versions",
	"/api/v0/query-playbooks",
	"/api/v0/query-playbooks/resolve",
	"/api/v0/relationships/catalog",
	"/api/v0/relationships/edges",
	"/api/v0/repositories",
	"/api/v0/repositories/by-language",
	"/api/v0/repositories/language-inventory",
	"/api/v0/repositories/{repo_id}/branches",
	"/api/v0/repositories/{repo_id}/content",
	"/api/v0/repositories/{repo_id}/context",
	"/api/v0/repositories/{repo_id}/coverage",
	"/api/v0/repositories/{repo_id}/freshness",
	"/api/v0/repositories/{repo_id}/stats",
	"/api/v0/repositories/{repo_id}/tree",
}

func TestD4WaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(d4WaveSpecKeys) != 45 {
		t.Fatalf("d4WaveSpecKeys holds %d keys, want the 45 served D4 keys (story omitted by disclosure)", len(d4WaveSpecKeys))
	}
}

// TestD4Native_ServesWaveSpecKeys pins that the native spec route
// serves the wave keys: authed callers get the dest-assembled document
// with every wave path, in both negotiated shapes, with or without a
// core origin configured.
func TestD4Native_ServesWaveSpecKeys(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"http://core.invalid", ""} {
		srv := NewServer(origin, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodGet, "/api/v0/openapi.json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("origin %q: status = %d, want 200", origin, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Errorf("origin %q: Content-Type = %q, want the JSON document shape", origin, got)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("origin %q: served spec is not valid JSON: %v", origin, err)
		}
		paths, _ := doc["paths"].(map[string]any)
		for _, path := range d4WaveSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks wave key %s", origin, path)
			}
		}
		if _, ok := paths["/api/v0/repositories/{repo_id}/story"]; ok {
			t.Errorf("origin %q: served spec carries the story key, want it omitted until the schema helper ports", origin)
		}
		if item, ok := paths["/api/v0/repositories"].(map[string]any); ok {
			if op, _ := item["get"].(map[string]any); op["operationId"] != "listRepositories" {
				t.Errorf("origin %q: repositories operationId = %v, want listRepositories", origin, op["operationId"])
			}
		}
		if item, ok := paths["/api/v0/ci-cd/run-correlations"].(map[string]any); ok {
			if op, _ := item["get"].(map[string]any); op["operationId"] != "listCICDRunCorrelations" {
				t.Errorf("origin %q: run-correlations operationId = %v, want listCICDRunCorrelations", origin, op["operationId"])
			}
		}
	}
}
