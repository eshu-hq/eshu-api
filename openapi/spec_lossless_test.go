// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// TestSpec_FamilySectionMatchesFragment pins lossless assembly: the served
// family path objects equal the ported fragments' parsed path objects, so
// the only reviewed delta to the pinned core fragments is the fragment files
// themselves (gated byte-identical by scripts/check-parity.sh).
func TestSpec_FamilySectionMatchesFragment(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)

	for _, path := range []string{
		"/api/v0/auth/sign-in-policy",
		"/api/v0/auth/admin/sign-in-policy",
		"/api/v0/admin/repository-retirements",
		"/api/v0/admin/collector-failures/acknowledge",
		"/api/v0/admin/collector-failures/acknowledgements/query",
		"/api/v0/admin/collector-failures/query",
		"/api/v0/auth/local/invitations/{invite_id}/revoke",
		"/api/v0/auth/admin/role-assignments/revoke",
		"/api/v0/auth/admin/idp-group-mappings/{mapping_ref}",
		"/api/v0/auth/admin/role-assignments",
		"/api/v0/auth/admin/roles",
		"/api/v0/auth/admin/idp-providers",
		"/api/v0/auth/admin/idp-group-mappings",
		"/api/v0/auth/admin/api-tokens",
		"/api/v0/auth/admin/audit/events",
		"/api/v0/auth/admin/audit/summary",
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/revert",
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/enable",
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/disable",
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/test-connection",
		"/api/v0/admin/replay",
		"/api/v0/admin/reopen",
		"/api/v0/admin/refinalize",
		"/api/v0/admin/recover-generations",
		"/api/v0/admin/workflow-retire-unclaimed",
		"/api/v0/supply-chain/impact/suppressions",
		"/api/v0/auth/setup-state",
		"/api/v0/auth/setup/admin",
		"/api/v0/auth/setup/claim",
		"/api/v0/auth/setup/mfa",
		"/api/v0/aws/runtime-drift/findings",
		"/api/v0/cloud/inventory",
		"/api/v0/cloud/runtime-drift/findings",
		"/api/v0/cloud/ssm-host-inventory",
		"/api/v0/code/bundles",
		"/api/v0/code/call-chain",
		"/api/v0/code/call-chain/compare",
		"/api/v0/code/call-graph/metrics",
		"/api/v0/code/complexity",
		"/api/v0/code/cypher",
		"/api/v0/code/dead-code",
		"/api/v0/code/dead-code/cross-repo",
		"/api/v0/code/dead-code/investigate",
		"/api/v0/code/divergence/findings",
		"/api/v0/code/divergence/investigate",
		"/api/v0/code/divergence/report",
		"/api/v0/code/flow/cfg-summary",
		"/api/v0/code/flow/pdg-summary",
		"/api/v0/code/flow/reaching-def",
		"/api/v0/code/flow/taint-path",
		"/api/v0/code/imports/investigate",
		"/api/v0/code/language-query",
		"/api/v0/code/quality/inspect",
		"/api/v0/code/relationships",
		"/api/v0/code/relationships/story",
		"/api/v0/code/routes/callers",
		"/api/v0/code/search",
		"/api/v0/code/security/secrets/investigate",
		"/api/v0/code/structure/inventory",
		"/api/v0/code/symbols/search",
		"/api/v0/code/topics/investigate",
		"/api/v0/code/visualize",
		"/api/v0/codeowners/ownership",
		"/api/v0/collector-extraction-readiness",
		"/api/v0/collector-extraction-readiness/{family}",
		"/api/v0/compare/environments",
		"/api/v0/component-extensions",
		"/api/v0/component-extensions/{component_id}/diagnostics",
		"/api/v0/dependencies",
		"/api/v0/entities/resolve",
		"/api/v0/entities/{entity_id}/context",
		"/api/v0/fact-schema-versions",
		"/api/v0/fact-schema-versions/{fact_kind}",
		"/api/v0/graph/entities",
		"/api/v0/iac/dead",
		"/api/v0/iac/management-status",
		"/api/v0/iac/management-status/explain",
		"/api/v0/iac/resources",
		"/api/v0/iac/terraform-import-plan/candidates",
		"/api/v0/iac/unmanaged-resources",
		"/api/v0/images",
		"/api/v0/images/tag-history",
		"/api/v0/impact/blast-radius",
		"/api/v0/impact/change-surface",
		"/api/v0/impact/change-surface/investigate",
		"/api/v0/impact/contracts",
		"/api/v0/impact/deployment-config-influence",
		"/api/v0/impact/developer-change-plan",
		"/api/v0/impact/entity-map",
		"/api/v0/impact/explain-dependency-path",
		"/api/v0/impact/pre-change",
		"/api/v0/impact/resource-investigation",
		"/api/v0/impact/trace-deployment-chain",
		"/api/v0/impact/trace-exposure-path",
		"/api/v0/impact/trace-resource-to-code",
		"/api/v0/incidents/{incident_id}/context",
		"/api/v0/investigations/drift/packet",
		"/api/v0/investigations/services/{service_name}",
		"/api/v0/investigations/supply-chain/impact/packet",
		"/api/v0/kubernetes/correlations",
		"/api/v0/metrics/timeseries",
		"/api/v0/observability/coverage/correlations",
		"/api/v0/replatforming/ownership-packets",
		"/api/v0/replatforming/plans",
		"/api/v0/replatforming/rollups",
		"/api/v0/replatforming/selectors",
		"/api/v0/repositories/{repo_id}/story",
		"/api/v0/repository/manifest-dependencies",
		"/api/v0/runtime-declarations/query",
		"/api/v0/search/semantic",
		"/api/v0/secrets-iam/identity-trust-chains",
		"/api/v0/secrets-iam/posture-gaps",
		"/api/v0/secrets-iam/posture-summary",
		"/api/v0/secrets-iam/privilege-posture-observations",
		"/api/v0/secrets-iam/secret-access-paths",
		"/api/v0/semantic/code-hints",
		"/api/v0/semantic/documentation-observations",
		"/api/v0/service-catalog/correlations",
		"/api/v0/services/{service_name}/api-surface/endpoints",
		"/api/v0/services/{service_name}/context",
		"/api/v0/services/{service_name}/downstream-consumers",
		"/api/v0/services/{service_name}/story",
		"/api/v0/status/answer-narration",
		"/api/v0/status/governance",
		"/api/v0/status/hosted-readiness",
		"/api/v0/surface-inventory",
		"/api/v0/terraform/config-state-drift/findings",
		"/api/v0/visualizations/derive",
		"/api/v0/work-items/evidence",
		"/api/v0/workloads/{workload_id}/context",
		"/api/v0/workloads/{workload_id}/story",
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
