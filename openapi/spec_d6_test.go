// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// D6 Status + SupplyChain reads spec proof (Refs eshu-hq/eshu#9669,
// part of #6707). The D6 lane ports twenty fragments into the served
// document: sixteen whole-file byte-identical ports (status collector
// readiness, status operations, operator control plane, semantic
// extraction, freshness causality, supply runtime context, advisory
// catalog, advisory evidence, container images, container-image
// aggregates, impact aggregates, SBOM attestations, SBOM aggregates,
// security alerts, security-alert aggregates, vulnerability-scanner
// contract), two line-verbatim finding subsets (impact findings,
// impact explain), one line-verbatim status subset (status reads
// without the governance, answer-narration, hosted-readiness
// (W8a-terminated with fragments disclosed core-side, see
// openapi/spec_w8a_test.go), and openapi.json entries), and one line-verbatim
// phase-2 index subset (status index plus the legacy index-status
// alias). Byte identity of every ported line rides
// scripts/check-parity.sh; these tests pin the served assembly around
// those bytes.
//
// New $refs the D6 lane introduces beyond the existing components
// block: schemas CoordinatorSource, TerraformStateSource. The generic
// TestSpec_LocalRefsResolve covers them without edits.

// TestSpec_ServesD6Families pins the native-spec contract for the D6
// families: the assembled document is valid JSON and carries every
// served D6 path key with its pinned operationId (where the fragment
// names one), while the disclosed omissions stay out.
func TestSpec_ServesD6Families(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantOp := range map[string]string{
		"/api/v0/collector-readiness":                          "getCollectorReadinessLegacy",
		"/api/v0/collectors":                                   "listCollectorsLegacy",
		"/api/v0/ingesters":                                    "listIngestersLegacy",
		"/api/v0/ingesters/{ingester}":                         "getIngesterStatusLegacy",
		"/api/v0/index-status":                                 "getIndexStatusLegacy",
		"/api/v0/status/index":                                 "getIndexStatus",
		"/api/v0/status/operations":                            "getOperations",
		"/api/v0/status/pipeline":                              "getPipelineStatus",
		"/api/v0/status/operator-control-plane":                "getOperatorControlPlane",
		"/api/v0/status/semantic-extraction":                   "getSemanticExtractionStatus",
		"/api/v0/status/freshness-causality":                   "getFreshnessCausality",
		"/api/v0/status/collector-readiness":                   "getCollectorReadiness",
		"/api/v0/supply-chain/advisories":                      "listAdvisoryCatalog",
		"/api/v0/supply-chain/advisories/evidence":             "listAdvisoryEvidence",
		"/api/v0/supply-chain/container-images/identities":     "listContainerImageIdentities",
		"/api/v0/supply-chain/impact/explain":                  "explainSupplyChainImpact",
		"/api/v0/supply-chain/impact/findings":                 "listSupplyChainImpactFindings",
		"/api/v0/supply-chain/impact/findings/count":           "countSupplyChainImpactFindings",
		"/api/v0/supply-chain/impact/inventory":                "getSupplyChainImpactInventory",
		"/api/v0/supply-chain/sbom-attestations/attachments":   "listSBOMAttestationAttachments",
		"/api/v0/supply-chain/security-alerts/reconciliations": "listSecurityAlertReconciliations",
		"/api/v0/supply-chain/vulnerabilities/{advisory_id}":   "getVulnerabilityDetail",
		"/api/v0/supply-chain/vulnerability-scanner/contract":  "getVulnerabilityScannerReadContract",
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
			got := op.(map[string]any)["operationId"]
			if got != wantOp {
				t.Errorf("%s %s operationId = %v, want %q", path, method, got, wantOp)
			}
		}
	}

	// The lossless-assembly property for the D6 keys: the served path
	// objects equal the ported fragments' parsed path objects, so the
	// only reviewed delta to the pinned core fragments is the fragment
	// files themselves (gated byte-identical or line-verbatim by
	// scripts/check-parity.sh).
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	for _, path := range []string{
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

// TestSpec_D6DisclosedOmissions pins the documented spec gap so it
// cannot silently widen: the impact packet (writer responder +
// shared ports) stays relay-only, so its key stays out of the served
// document. The governance, answer-narration, and hosted-readiness
// aspects left this list when the W8a lane terminated them natively
// with their fragments disclosed core-side (see
// openapi/spec_w8a_test.go); Compare left earlier with its gap-04
// lane (see openapi/spec_compare_test.go).
func TestSpec_D6DisclosedOmissions(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())

	for _, path := range []string{
		"/api/v0/investigations/supply-chain/impact/packet",
	} {
		if _, ok := served[path]; ok {
			t.Errorf("served spec carries %s, want it relay-only until its pin lands", path)
		}
	}
}
