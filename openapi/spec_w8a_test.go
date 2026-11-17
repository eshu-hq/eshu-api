// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// W8a full-surface spec proof (Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#100). The 16 W8a routes mount natively in their wave gate and their Decision 5(c) fragments now ride the served document; the D13 row-7 full-surface lane claims the keys the satellite termination lane disclosed absent.
//
// The lossless-assembly property holds for the lane keys: the
// served path objects equal the ported fragments parsed path
// objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated
// byte-identical or line-verbatim by scripts/check-parity.sh).
func TestSpec_ServesW8aWave(t *testing.T) {
	assertServedPaths(t, map[string]map[string]string{
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
	})

	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)
	served := decodePaths(t, Spec())
	for _, path := range []string{
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
