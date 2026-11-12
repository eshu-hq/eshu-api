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

// D6 Status + SupplyChain served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, part of #6707). The openapi package tests pin the
// assembled document; these tests pin that the native openapi.json
// route serves it to keyed callers with the wave path keys present —
// for both a configured and an unconfigured core origin — while the
// key gate still fronts the route (the standing no-new-public-surface
// decision for the assembled spec).

// d6WaveSpecKeys are the path keys the D6 wave adds to the served
// document: one entry per served wave route. The impact packet and
// the mux-level openapi.json self entry keep their own posture:
// their keys stay out of this fragment set, so no served key
// duplicates a relayed route. The former still-relayed Status
// aspects (governance, answer-narration, hosted-readiness) moved to
// the W8a lane with their fragments disclosed core-side (see
// api/w8a_openapi_test.go). Compare carries no served key: its
// gap-04 lane mounts natively with the fragment disclosed core-side
// (see openapi/spec_compare_test.go).
var d6WaveSpecKeys = []string{
	"/api/v0/collector-readiness",
	"/api/v0/collectors",
	"/api/v0/ingesters",
	"/api/v0/ingesters/{ingester}",
	"/api/v0/index-status",
	"/api/v0/status/collector-readiness",
	"/api/v0/status/collectors",
	"/api/v0/status/freshness-causality",
	"/api/v0/status/index",
	"/api/v0/status/ingesters",
	"/api/v0/status/ingesters/{ingester}",
	"/api/v0/status/operations",
	"/api/v0/status/operator-control-plane",
	"/api/v0/status/pipeline",
	"/api/v0/status/semantic-extraction",
	"/api/v0/supply-chain/advisories",
	"/api/v0/supply-chain/advisories/evidence",
	"/api/v0/supply-chain/container-images/identities",
	"/api/v0/supply-chain/container-images/identities/count",
	"/api/v0/supply-chain/container-images/identities/inventory",
	"/api/v0/supply-chain/impact/explain",
	"/api/v0/supply-chain/impact/findings",
	"/api/v0/supply-chain/impact/findings/count",
	"/api/v0/supply-chain/impact/inventory",
	"/api/v0/supply-chain/sbom-attestations/attachments",
	"/api/v0/supply-chain/sbom-attestations/attachments/count",
	"/api/v0/supply-chain/sbom-attestations/attachments/inventory",
	"/api/v0/supply-chain/security-alerts/reconciliations",
	"/api/v0/supply-chain/security-alerts/reconciliations/count",
	"/api/v0/supply-chain/security-alerts/reconciliations/inventory",
	"/api/v0/supply-chain/vulnerabilities/{advisory_id}",
	"/api/v0/supply-chain/vulnerability-scanner/contract",
}

// d6RelayedSpecKeys are the path keys the D6 lane deliberately does
// NOT serve: the impact packet keeps its core-served posture via the
// relay, so its key must not appear in the dest-assembled document.
// The governance, answer-narration, and hosted-readiness aspects left
// this list when the W8a lane terminated them natively with their
// fragments disclosed core-side (see api/w8a_openapi_test.go);
// Compare left earlier when its gap-04 lane mounted natively the
// same way (see openapi/spec_compare_test.go).
var d6RelayedSpecKeys = []string{
	"/api/v0/investigations/supply-chain/impact/packet",
}

func TestD6WaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(d6WaveSpecKeys) != 32 {
		t.Fatalf("d6WaveSpecKeys holds %d keys, want the 32 served D6 keys (the impact packet omitted by disclosure)", len(d6WaveSpecKeys))
	}
}

// TestD6Native_ServesWaveSpecKeys pins that the native spec route
// serves the wave keys and omits the relayed keys: authed callers get
// the dest-assembled document with every wave path, with or without a
// core origin configured.
func TestD6Native_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range d6WaveSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks wave key %s", origin, path)
			}
		}
		for _, path := range d6RelayedSpecKeys {
			if _, ok := paths[path]; ok {
				t.Errorf("origin %q: served spec carries relayed key %s, want it out", origin, path)
			}
		}
		if item, ok := paths["/api/v0/status/pipeline"].(map[string]any); ok {
			if op, _ := item["get"].(map[string]any); op["operationId"] != "getPipelineStatus" {
				t.Errorf("origin %q: pipeline operationId = %v, want getPipelineStatus", origin, op["operationId"])
			}
		}
		if item, ok := paths["/api/v0/supply-chain/impact/findings"].(map[string]any); ok {
			if op, _ := item["get"].(map[string]any); op["operationId"] != "listSupplyChainImpactFindings" {
				t.Errorf("origin %q: findings operationId = %v, want listSupplyChainImpactFindings", origin, op["operationId"])
			}
		}
	}
}

// TestD6OpenAPIStillKeyGated pins that the assembled spec route stays
// behind the key gate: a credential-less caller gets the standing 401
// even though the document is fully dest-served.
func TestD6OpenAPIStillKeyGated(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v0/openapi.json", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want the standing 401", rec.Code)
	}
}
