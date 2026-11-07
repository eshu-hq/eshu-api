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

// D5 wiring probes: the D2-D4 terminated GETs against the served
// docs/GET shapes (Refs eshu-hq/eshu#9669, part of #6707). This lane
// terminates nothing and ports no fragments: it probes the wiring of
// the D2 Leaf B, D3 admin/identity/recovery, and D4 Native-next GETs
// against what the router serves and what the served openapi.json
// promises — methods, paths, operationIds, response shapes — and fails
// on any drift the lane contracts do not disclose.
//
// TDD record: the strict bijection (every wave GET has a spec get
// entry, zero carve-outs) ran RED first on exactly the four disclosed
// absences below (/health, /docs, /redoc mux-level entries, the
// repositories story omission) with the other 54 GETs present; this
// file pins that outcome. It extends the wave-gated pattern — frozen
// tables mirrored off the wave tables, behavior proved against the
// served document — and invents no new pattern: the probe reuses
// leafbWave, d3Wave, and d4Wave directly, so a wave change fails here
// until the probe and the owning lane move together.
//
// Production binding: none. This lane adds no routes, no fragments,
// no latency surface, and no telemetry: it reads the served document
// through the existing openapi.json route exactly as a keyed caller
// would. Rollback is deletion of this file and
// contracts/d5-wiring-probes.md with zero serving change.

// d5ProbeGET is one probed wiring: the served path key plus the
// operationId the fragment promises for its get method.
type d5ProbeGET struct {
	Path        string
	OperationID string
}

// d5ProbeGETs is the frozen D2-D4 terminated GET surface that carries
// a served spec get-entry: 9 D2 reads (the openapi.json self entry,
// capabilities, service intelligence, the two sign-in-policy reads,
// the auth-providers picker, and the three provider-config reads), 8
// D3 reads (six admin-identity GETs plus the two governance-audit
// GETs), and 37 D4 reads (CICD, Documentation, Infra, PackageRegistry,
// Repositories, Evidence, InvestigationWorkflows, Playbooks minus the
// disclosed story omission). Order follows the waves (D2, D3, D4);
// within D4, ruling family order.
var d5ProbeGETs = []d5ProbeGET{
	{Path: "/api/v0/openapi.json", OperationID: "getOpenAPISpec"},
	{Path: "/api/v0/capabilities", OperationID: "listCapabilities"},
	{Path: "/api/v0/services/{service_name}/intelligence-report", OperationID: "getServiceIntelligenceReport"},
	{Path: "/api/v0/auth/providers", OperationID: "listAuthProviders"},
	{Path: "/api/v0/auth/sign-in-policy", OperationID: "getPublicSignInPolicy"},
	{Path: "/api/v0/auth/admin/sign-in-policy", OperationID: "getAdminSignInPolicy"},
	{Path: "/api/v0/auth/admin/provider-configs", OperationID: "listAdminProviderConfigs"},
	{Path: "/api/v0/auth/admin/provider-configs/{provider_config_id}", OperationID: "getAdminProviderConfig"},
	{Path: "/api/v0/auth/admin/provider-configs/{provider_config_id}/revisions", OperationID: "listAdminProviderConfigRevisions"},
	{Path: "/api/v0/auth/admin/api-tokens", OperationID: "listAdminAPITokens"},
	{Path: "/api/v0/auth/admin/idp-group-mappings", OperationID: "listAdminIdPGroupMappings"},
	{Path: "/api/v0/auth/admin/idp-providers", OperationID: "listAdminIdPProviders"},
	{Path: "/api/v0/auth/admin/role-assignments", OperationID: "listAdminRoleAssignments"},
	{Path: "/api/v0/auth/admin/roles", OperationID: "listAdminRoles"},
	{Path: "/api/v0/auth/local/invitations", OperationID: "listAdminInvitations"},
	{Path: "/api/v0/auth/admin/audit/events", OperationID: "listAdminAuditEvents"},
	{Path: "/api/v0/auth/admin/audit/summary", OperationID: "summarizeAdminAuditEvents"},
	{Path: "/api/v0/catalog", OperationID: "listCatalog"},
	{Path: "/api/v0/ci-cd/run-correlations", OperationID: "listCICDRunCorrelations"},
	{Path: "/api/v0/ci-cd/run-correlations/count", OperationID: "countCICDRunCorrelations"},
	{Path: "/api/v0/ci-cd/run-correlations/inventory", OperationID: "getCICDRunCorrelationInventory"},
	{Path: "/api/v0/cloud/resources", OperationID: "listCloudResources"},
	{Path: "/api/v0/documentation/evidence-packets/{packet_id}/freshness", OperationID: "checkDocumentationEvidencePacketFreshness"},
	{Path: "/api/v0/documentation/facts", OperationID: "listDocumentationFacts"},
	{Path: "/api/v0/documentation/findings", OperationID: "listDocumentationFindings"},
	{Path: "/api/v0/documentation/findings/count", OperationID: "countDocumentationFindings"},
	{Path: "/api/v0/documentation/findings/inventory", OperationID: "getDocumentationFindingInventory"},
	{Path: "/api/v0/documentation/findings/{finding_id}/evidence-packet", OperationID: "getDocumentationEvidencePacket"},
	{Path: "/api/v0/ecosystem/overview", OperationID: "getEcosystemOverview"},
	{Path: "/api/v0/evidence/admission-decisions", OperationID: "listAdmissionDecisions"},
	{Path: "/api/v0/evidence/bundle", OperationID: "getLiveEvidenceBundle"},
	{Path: "/api/v0/evidence/relationships/{resolved_id}", OperationID: "getRelationshipEvidence"},
	{Path: "/api/v0/infra/resources/count", OperationID: "countInfraResources"},
	{Path: "/api/v0/infra/resources/inventory", OperationID: "getInfraResourceInventory"},
	{Path: "/api/v0/investigation-workflows", OperationID: "listInvestigationWorkflows"},
	{Path: "/api/v0/investigations/deployable-unit/packet", OperationID: "getDeployableUnitPacket"},
	{Path: "/api/v0/package-registry/correlations", OperationID: "listPackageRegistryCorrelations"},
	{Path: "/api/v0/package-registry/dependencies", OperationID: "listPackageRegistryDependencies"},
	{Path: "/api/v0/package-registry/dependency-chains", OperationID: "listPackageRegistryDependencyChains"},
	{Path: "/api/v0/package-registry/packages", OperationID: "listPackageRegistryPackages"},
	{Path: "/api/v0/package-registry/packages/count", OperationID: "countPackageRegistryPackages"},
	{Path: "/api/v0/package-registry/packages/inventory", OperationID: "getPackageRegistryPackageInventory"},
	{Path: "/api/v0/package-registry/versions", OperationID: "listPackageRegistryVersions"},
	{Path: "/api/v0/query-playbooks", OperationID: "listQueryPlaybooks"},
	{Path: "/api/v0/repositories", OperationID: "listRepositories"},
	{Path: "/api/v0/repositories/by-language", OperationID: "listRepositoriesByLanguage"},
	{Path: "/api/v0/repositories/language-inventory", OperationID: "getRepositoryLanguageInventory"},
	{Path: "/api/v0/repositories/{repo_id}/branches", OperationID: "getRepositoryBranches"},
	{Path: "/api/v0/repositories/{repo_id}/content", OperationID: "getRepositoryContent"},
	{Path: "/api/v0/repositories/{repo_id}/context", OperationID: "getRepositoryContext"},
	{Path: "/api/v0/repositories/{repo_id}/coverage", OperationID: "getRepositoryCoverage"},
	{Path: "/api/v0/repositories/{repo_id}/freshness", OperationID: "getRepositoryFreshness"},
	{Path: "/api/v0/repositories/{repo_id}/stats", OperationID: "getRepositoryStats"},
	{Path: "/api/v0/repositories/{repo_id}/tree", OperationID: "getRepositoryTree"},
}

// d5ExpectedAbsent pins the terminated GETs with no served spec
// get-entry, each with the lane contract that discloses it. A newly
// absent GET fails the shape test until it lands here with its
// disclosure; a newly present one fails here until the probe table
// above claims it.
var d5ExpectedAbsent = map[string]string{
	"GET /health":       "mux-health probe with a dest-owned shape, never an OpenAPI path (contracts/leafb-native-now.md)",
	"GET /api/v0/docs":  "compiled doc-UI shell, never an OpenAPI path (contracts/leafb-native-now.md; serving pinned by TestLeafBPublicDocUI)",
	"GET /api/v0/redoc": "compiled doc-UI shell, never an OpenAPI path (contracts/leafb-native-now.md; serving pinned by TestLeafBPublicDocUI)",
	"GET /api/v0/repositories/{repo_id}/story": "spec entry waits for the schema.EvidenceBoundaries helper port; the route terminates regardless (contracts/d4-native-next.md)",
}

// d5WaveGETs collects every Method+Path GET across the three
// terminated waves in wave order.
func d5WaveGETs() []string {
	var got []string
	for _, e := range leafbWave {
		if e.Method == http.MethodGet {
			got = append(got, e.Method+" "+e.Path)
		}
	}
	for _, e := range d3Wave {
		if e.Method == http.MethodGet {
			got = append(got, e.Method+" "+e.Path)
		}
	}
	for _, e := range d4Wave {
		if e.Method == http.MethodGet {
			got = append(got, e.Method+" "+e.Path)
		}
	}
	return got
}

// d5ServedPaths fetches the served openapi.json exactly as a keyed
// caller would and returns its decoded paths object.
func d5ServedPaths(t *testing.T, origin string) map[string]any {
	t.Helper()

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
	if paths == nil {
		t.Fatalf("origin %q: served spec has no paths object", origin)
	}
	return paths
}

// TestD5ProbeFrozen pins the probe surface against the wave tables: 58
// terminated GETs across D2-D4, of which 54 carry a served spec entry
// and 4 stay absent by disclosure. A wave change (route added,
// removed, renamed) fails here until the probe table, the expected
// absences, and the owning lane move together.
func TestD5ProbeFrozen(t *testing.T) {
	t.Parallel()

	waveGETs := d5WaveGETs()
	if len(waveGETs) != 58 {
		t.Fatalf("wave GETs = %d, want 58 (12 D2 + 8 D3 + 38 D4): move the probe with the owning lane", len(waveGETs))
	}
	if len(d5ProbeGETs) != 54 {
		t.Fatalf("probe GETs = %d, want 54: move the probe with the owning lane", len(d5ProbeGETs))
	}
	if len(d5ExpectedAbsent) != 4 {
		t.Fatalf("expected-absent GETs = %d, want 4: each absence needs its disclosing contract", len(d5ExpectedAbsent))
	}

	probed := make(map[string]bool, len(d5ProbeGETs))
	for _, p := range d5ProbeGETs {
		probed["GET "+p.Path] = true
	}
	for _, key := range waveGETs {
		if probed[key] {
			continue
		}
		if _, ok := d5ExpectedAbsent[key]; !ok {
			t.Errorf("wave GET %s is neither probed nor disclosed absent: claim it or disclose it", key)
		}
	}
	for key := range d5ExpectedAbsent {
		if probed[key] {
			t.Errorf("GET %s is both probed and disclosed absent: pick one", key)
		}
		found := false
		for _, w := range waveGETs {
			if w == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("disclosed-absent GET %s matches no wave GET: drop it with the owning lane", key)
		}
	}
}

// TestD5Wiring_ServedGETShapes pins the forward wiring: every probed
// GET has its served path key with the get method, the promised
// operationId, and a 200 response entry — for both a configured and
// an unconfigured core origin, since the document serves without a
// core fetch.
func TestD5Wiring_ServedGETShapes(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"http://core.invalid", ""} {
		paths := d5ServedPaths(t, origin)
		for _, p := range d5ProbeGETs {
			item, ok := paths[p.Path].(map[string]any)
			if !ok {
				t.Errorf("origin %q: served spec lacks path key %s", origin, p.Path)
				continue
			}
			op, ok := item["get"].(map[string]any)
			if !ok {
				t.Errorf("origin %q: served spec lacks get under %s", origin, p.Path)
				continue
			}
			if got, _ := op["operationId"].(string); got != p.OperationID {
				t.Errorf("origin %q: %s get operationId = %q, want %q", origin, p.Path, got, p.OperationID)
			}
			resps, ok := op["responses"].(map[string]any)
			if !ok || len(resps) == 0 {
				t.Errorf("origin %q: %s get has no responses shape", origin, p.Path)
				continue
			}
			two, ok := resps["200"].(map[string]any)
			if !ok || len(two) == 0 {
				t.Errorf("origin %q: %s get has no 200 response shape", origin, p.Path)
			}
		}
	}
}

// TestD5Wiring_ExpectedAbsentStaysAbsent pins the disclosed drift:
// the mux-level GETs never gain a spec path key, and the story route
// never gains a spec get-entry, until its helper ports as a component
// and the probe table claims it.
func TestD5Wiring_ExpectedAbsentStaysAbsent(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"http://core.invalid", ""} {
		paths := d5ServedPaths(t, origin)
		for _, key := range []string{"GET /health", "GET /api/v0/docs", "GET /api/v0/redoc"} {
			path := key[len("GET "):]
			if _, ok := paths[path]; ok {
				t.Errorf("origin %q: served spec gained mux-level key %s: claim it in the probe or revert the fragment", origin, path)
			}
		}
		const story = "/api/v0/repositories/{repo_id}/story"
		if item, ok := paths[story].(map[string]any); ok {
			if _, has := item["get"]; has {
				t.Errorf("origin %q: served spec gained the story get-entry: claim it in the probe table", origin)
			}
		}
	}
}

// TestD5Wiring_NoUndocumentedGETs pins the reverse wiring: every get
// operation the served document promises maps to a probed terminated
// GET. A newly landed GET fragment fails here until the probe table
// claims it — the document can never promise what the probe does not
// watch. The D6 lane claims its 29 keys in its owning table
// (d6WaveSpecKeys, watched by the D6 conformance/openapi/spec suites),
// the D8 tier-C lane claims its 4 served GET keys in its owning table
// (d8WaveSpecKeys, watched by the D8 conformance/openapi/spec suites),
// and the D9 OIDC tier-C lane claims its 2 served GET keys in its
// owning table (d9WaveSpecKeys, watched by the D9
// conformance/openapi/spec suites), and the D9 GitHub tier-C lane
// claims its 2 served GET keys in its owning table
// (d9GitHubWaveSpecKeys, watched by the D9 GitHub
// conformance/openapi/spec suites), so the reverse check unions all
// five lanes' claim tables; the D2-D4 frozen counts in
// TestD5ProbeFrozen stay untouched.
func TestD5Wiring_NoUndocumentedGETs(t *testing.T) {
	t.Parallel()

	probed := make(map[string]bool, len(d5ProbeGETs)+len(d6WaveSpecKeys)+len(d8WaveSpecKeys)+len(d9WaveSpecKeys)+len(d9GitHubWaveSpecKeys))
	for _, p := range d5ProbeGETs {
		probed[p.Path] = true
	}
	for _, path := range d6WaveSpecKeys {
		probed[path] = true
	}
	for path := range d8WaveSpecKeys {
		probed[path] = true
	}
	for path := range d9WaveSpecKeys {
		probed[path] = true
	}
	for path := range d9GitHubWaveSpecKeys {
		probed[path] = true
	}
	for _, origin := range []string{"http://core.invalid", ""} {
		paths := d5ServedPaths(t, origin)
		for path, raw := range paths {
			item, _ := raw.(map[string]any)
			if _, ok := item["get"].(map[string]any); !ok {
				continue
			}
			if !probed[path] {
				t.Errorf("origin %q: served get %s has no probe entry: claim it or revert the fragment", origin, path)
			}
		}
	}
}
