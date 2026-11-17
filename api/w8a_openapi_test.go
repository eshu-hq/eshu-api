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

// W8a satellite termination served-spec posture (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#100). The 16 W8a routes mount natively,
// but their Decision 5(c) fragments stay core-side for this lane (the
// same disclosed-absence posture the Compare gap-04 lane records in
// contracts/compare-termination.md): a byte-identical satellite port
// cannot concatenate mid-list, and a re-shaped satellite subset
// would break the verbatim-port rule scripts/check-parity.sh gates.
// The follow-up spec-port lane claims the keys when core re-shapes
// the fragments or the lane ports them entry-shaped; until then no
// served key duplicates a relayed route, and no mounted route
// promises what the document does not carry.

// w8aWaveSpecKeys are the path keys the W8a lane adds to the served
// document: one entry per natively mounted wave route.
var w8aWaveSpecKeys = []string{
	"/api/v0/secrets-iam/identity-trust-chains",
	"/api/v0/secrets-iam/privilege-posture-observations",
	"/api/v0/secrets-iam/secret-access-paths",
	"/api/v0/secrets-iam/posture-gaps",
	"/api/v0/secrets-iam/posture-summary",
	"/api/v0/status/governance",
	"/api/v0/status/answer-narration",
	"/api/v0/status/hosted-readiness",
	"/api/v0/aws/runtime-drift/findings",
	"/api/v0/replatforming/selectors",
	"/api/v0/replatforming/rollups",
	"/api/v0/replatforming/plans",
	"/api/v0/replatforming/ownership-packets",
	"/api/v0/metrics/timeseries",
	"/api/v0/search/semantic",
	"/api/v0/terraform/config-state-drift/findings",
}

// TestW8aDisclosedSpecKeyCount pins the W8a disclosed set: one entry
// per natively mounted wave route. A wave change moves this list
// with the ownership note, the fragments, and the wave mirror
// together.
func TestW8aWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(w8aWaveSpecKeys) != 16 {
		t.Fatalf("w8aWaveSpecKeys holds %d keys, want the 16 served W8a keys (one per wave route): move the key with the owning lane", len(w8aWaveSpecKeys))
	}
	if len(w8aWaveSpecKeys) != len(w8aWave) {
		t.Fatalf("w8aWaveSpecKeys holds %d keys for %d wave routes, want one served key per wave route", len(w8aWaveSpecKeys), len(w8aWave))
	}
}

// w8aWaveSpecOperations pins the served method plus operationId per
// W8a path key, mirroring the pinned core fragments.
var w8aWaveSpecOperations = map[string]map[string]string{
	"/api/v0/secrets-iam/identity-trust-chains":          {"get": "listSecretsIAMIdentityTrustChains"},
	"/api/v0/secrets-iam/privilege-posture-observations": {"get": "listSecretsIAMPrivilegePostureObservations"},
	"/api/v0/secrets-iam/secret-access-paths":            {"get": "listSecretsIAMSecretAccessPaths"},
	"/api/v0/secrets-iam/posture-gaps":                   {"get": "listSecretsIAMPostureGaps"},
	"/api/v0/secrets-iam/posture-summary":                {"get": "countSecretsIAMPosture"},
	"/api/v0/status/governance":                          {"get": "getHostedGovernanceStatus"},
	"/api/v0/status/answer-narration":                    {"get": "getAnswerNarrationStatus"},
	"/api/v0/status/hosted-readiness":                    {"get": "getHostedReadiness"},
	"/api/v0/aws/runtime-drift/findings":                 {"post": "listAWSRuntimeDriftFindings"},
	"/api/v0/replatforming/selectors":                    {"get": "listReplatformingSelectors"},
	"/api/v0/replatforming/rollups":                      {"post": "rollupReplatformingReadiness"},
	"/api/v0/replatforming/plans":                        {"post": "composeReplatformingPlan"},
	"/api/v0/replatforming/ownership-packets":            {"post": "composeReplatformingOwnershipPackets"},
	"/api/v0/metrics/timeseries":                         {"get": "getMetricsTimeSeries"},
	"/api/v0/search/semantic":                            {"post": "searchSemanticContext"},
	"/api/v0/terraform/config-state-drift/findings":      {"post": "listTerraformConfigStateDriftFindings"},
}

// TestW8aNative_ServesWaveSpecKeys pins that the native spec route
// serves the W8a keys: authed callers get the dest-assembled
// document with every W8a path key and its pinned operationId, with
// or without a core origin configured.
func TestW8aNative_ServesWaveSpecKeys(t *testing.T) {
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
		body, _ := io.ReadAll(rec.Result().Body)
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("origin %q: served spec is not valid JSON: %v", origin, err)
		}
		paths, _ := doc["paths"].(map[string]any)
		for _, path := range w8aWaveSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks W8a wave key %s", origin, path)
			}
		}
		for path, wantOps := range w8aWaveSpecOperations {
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
