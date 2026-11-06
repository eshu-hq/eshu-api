// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"encoding/json"
	"testing"
)

// D2 Leaf B Native-now spec proof (Refs eshu-hq/eshu#9669, part of
// #6707). The wave ports 13 new path keys into the served document:
// the openapi.json self entry, capabilities, five CLI support routes,
// ask, the service intelligence report, auth providers, and the three
// provider-config read paths (whose list/detail keys also carry the
// create/update POSTs, per the core OpenAPI convention). Byte identity
// of every ported line with the pinned core sources rides
// scripts/check-parity.sh; these tests pin the served assembly around
// those bytes (valid JSON, pinned operationIds, lossless fragment
// embedding, resolvable local $refs).
//
// New $refs the wave introduces beyond the existing components block:
// Unauthorized (ask), NotImplemented and parameters/ServiceName
// (intelligence-report), AdminProviderConfig and
// AdminProviderConfigWriteRequest (provider-config reads). The generic
// TestSpec_LocalRefsResolve covers them without edits; the entries
// above are named here so a reader need not diff the fragments.

// TestSpec_ServesLeafBWaveFamilies pins the dest-served openapi.json
// for the D2 wave: the assembled document is valid JSON and carries
// every wave path key with its pinned operationId, served from
// compiled constants with no core fetch.
func TestSpec_ServesLeafBWaveFamilies(t *testing.T) {
	t.Parallel()

	var doc map[string]any
	if err := json.Unmarshal([]byte(Spec()), &doc); err != nil {
		t.Fatalf("Spec() is not valid JSON: %v", err)
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("Spec() paths = %T, want object", doc["paths"])
	}

	for path, want := range map[string]map[string]string{
		"/api/v0/openapi.json": {"get": "getOpenAPISpec"},
		"/api/v0/capabilities": {"get": "listCapabilities"},
		"/api/v0/answer-quality/score": {
			"post": "scoreAnswerQuality",
		},
		"/api/v0/investigation/packet/render": {
			"post": "renderInvestigationPacket",
		},
		"/api/v0/report/bundle/capture": {
			"post": "captureReportBundle",
		},
		"/api/v0/report/bundle/validate": {
			"post": "validateReportBundle",
		},
		"/api/v0/service/report/compose": {
			"post": "composeServiceReport",
		},
		"/api/v0/ask": {"post": "ask"},
		"/api/v0/services/{service_name}/intelligence-report": {
			"get": "getServiceIntelligenceReport",
		},
		"/api/v0/auth/providers": {"get": "listAuthProviders"},
		"/api/v0/auth/admin/provider-configs": {
			"get":  "listAdminProviderConfigs",
			"post": "createAdminProviderConfig",
		},
		"/api/v0/auth/admin/provider-configs/{provider_config_id}": {
			"get":  "getAdminProviderConfig",
			"post": "updateAdminProviderConfig",
		},
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/revisions": {
			"get": "listAdminProviderConfigRevisions",
		},
	} {
		item, ok := paths[path].(map[string]any)
		if !ok {
			t.Errorf("served spec lacks %s", path)
			continue
		}
		for operation, wantID := range want {
			if got := operationID(item, operation); got != wantID {
				t.Errorf("%s %s operationId = %q, want %s", path, operation, got, wantID)
			}
		}
	}
}

// TestSpec_LeafBFragmentMatchesFragment pins lossless assembly for
// the wave: every wave path key the served document carries equals the
// ported fragment entry (the same shape TestSpec_FamilySectionMatchesFragment
// pins for the earlier lanes).
func TestSpec_LeafBFragmentMatchesFragment(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)

	for _, path := range []string{
		"/api/v0/openapi.json",
		"/api/v0/capabilities",
		"/api/v0/answer-quality/score",
		"/api/v0/investigation/packet/render",
		"/api/v0/report/bundle/capture",
		"/api/v0/report/bundle/validate",
		"/api/v0/service/report/compose",
		"/api/v0/ask",
		"/api/v0/services/{service_name}/intelligence-report",
		"/api/v0/auth/providers",
		"/api/v0/auth/admin/provider-configs",
		"/api/v0/auth/admin/provider-configs/{provider_config_id}",
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/revisions",
	} {
		want, ok := fragment[path]
		if !ok {
			t.Fatalf("fragment lacks %s", path)
		}
		got, ok := served[path]
		if !ok {
			t.Fatalf("served spec lacks %s", path)
		}
		if !jsonEqual(got, want) {
			t.Errorf("served %s differs from the ported fragment", path)
		}
	}
}
