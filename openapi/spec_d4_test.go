// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// D4 Native-next spec proof (Refs eshu-hq/eshu#9669, part of #6707).
// The D4 lane ports seventeen fragments into the served document:
// thirteen whole-file byte-identical ports (CICD routes + aggregates,
// documentation + finding aggregates, cloud routes, infrastructure
// routes + aggregates, package-registry + aggregates, branches,
// stats/coverage, freshness, evidence bundle, investigation workflows,
// playbooks) and three line-verbatim subsets (evidence routes without
// the dangling 501, the deployable-unit packet entry, the repository
// routes without the mux-health and schema-interpolated story entries).
// Byte identity of every ported line rides scripts/check-parity.sh;
// these tests pin the served assembly around those bytes.
//
// New $refs the D4 lane introduces beyond the existing components
// block: parameters/RepoId; schemas CollectorReadinessEnvelope,
// Repository, RepositoryRef, CatalogWorkload,
// InvestigationEvidencePacket, Relationship, ActiveWorkSource;
// responses/GatewayTimeout. The generic
// TestSpec_LocalRefsResolve covers them without edits.

// TestSpec_ServesD4Families pins the native-spec contract for the D4
// families: the assembled document is valid JSON and carries every
// served D4 path key with its pinned operationId (where the fragment
// names one), while the disclosed omissions stay out.
func TestSpec_ServesD4Families(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantOp := range map[string]string{
		"/api/v0/catalog":                      "listCatalog",
		"/api/v0/ci-cd/run-correlations":       "listCICDRunCorrelations",
		"/api/v0/cloud/resources":              "listCloudResources",
		"/api/v0/documentation/facts":          "listDocumentationFacts",
		"/api/v0/ecosystem/overview":           "getEcosystemOverview",
		"/api/v0/evidence/admission-decisions": "listAdmissionDecisions",
		"/api/v0/evidence/bundle":              "getLiveEvidenceBundle",
		"/api/v0/investigation-workflows":      "listInvestigationWorkflows",
		"/api/v0/package-registry/packages":    "listPackageRegistryPackages",
		"/api/v0/query-playbooks":              "listQueryPlaybooks",
		"/api/v0/repositories":                 "listRepositories",
	} {
		item, ok := served[path].(map[string]any)
		if !ok {
			t.Errorf("paths lacks %s: %v", path, keys(served))
			continue
		}
		if len(item) != 1 {
			t.Errorf("%s serves %d methods, want exactly the one the fragment documents", path, len(item))
			continue
		}
		for method, op := range item {
			got, _ := op.(map[string]any)["operationId"]
			if got != wantOp {
				t.Errorf("%s %s operationId = %v, want %q", path, method, got, wantOp)
			}
		}
	}

	// The lossless-assembly property for the D4 keys: the served path
	// objects equal the ported fragments' parsed path objects, so the
	// only reviewed delta to the pinned core fragments is the fragment
	// files themselves (gated byte-identical or line-verbatim by
	// scripts/check-parity.sh).
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	for _, path := range []string{
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

// TestSpec_D4DisclosedOmissions pins the three documented spec gaps so
// they cannot silently widen: the story key (schema-interpolated entry),
// the admission-decisions 501 (dangling core $ref), and the relay-only
// packets sharing the investigations file.
func TestSpec_D4DisclosedOmissions(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())

	if _, ok := served["/api/v0/repositories/{repo_id}/story"]; ok {
		t.Errorf("served spec carries the story key, want it omitted until the schema helper ports as a component")
	}
	if _, ok := served["/health"]; ok {
		t.Errorf("served spec carries the mux-health entry, want it core-side only (dest serves its own probe shape)")
	}
	if _, ok := served["/api/v0/investigations/supply-chain/impact/packet"]; ok {
		t.Errorf("served spec carries the supply-chain packet, want it relay-only until its lane")
	}
	if _, ok := served["/api/v0/investigations/drift/packet"]; ok {
		t.Errorf("served spec carries the drift packet, want it relay-only until its lane")
	}
	if _, ok := served["/api/v0/investigations/services/{service_name}"]; ok {
		t.Errorf("served spec carries the services packet, want it relay-only until its lane")
	}

	// The admission-decisions entry serves without the dangling 501:
	// every served response $ref must resolve (also covered by
	// TestSpec_LocalRefsResolve), so assert the entry's 501 is absent
	// rather than dangling.
	item, ok := served["/api/v0/evidence/admission-decisions"].(map[string]any)
	if !ok {
		t.Fatalf("served spec lacks the admission-decisions key")
	}
	get, _ := item["get"].(map[string]any)
	responses, _ := get["responses"].(map[string]any)
	if _, ok := responses["501"]; ok {
		t.Errorf("admission-decisions serves a 501, want it omitted until core defines the response")
	}
}
