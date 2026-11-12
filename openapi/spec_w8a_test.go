// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// W8a satellite termination spec posture (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#100). The 16 W8a routes mount natively
// in their wave gate, but their Decision 5(c) fragments stay
// core-side for this lane: each fragment is shaped as a mid-list
// entry of its own assembled document, so a byte-identical satellite
// port cannot concatenate mid-list — and a re-shaped satellite
// subset would break the verbatim-port rule
// scripts/check-parity.sh gates. The routes are therefore disclosed
// spec absences (the same posture the Compare gap-04 lane records),
// pinned below. A follow-up spec-port lane claims the keys when core
// re-shapes a fragment or the lane ports it entry-shaped; until then
// the served document is unchanged by this lane and no mounted route
// promises what the document does not carry (see
// contracts/w8a-termination.md).

// TestSpec_W8aStaysDisclosedAbsent pins the W8a spec posture: the
// served document carries none of the 16 W8a keys while the
// fragments stay core-side. Lifting an absence (porting the
// fragment) belongs to the follow-up spec-port lane, not to the
// relay-leg termination.
func TestSpec_W8aStaysDisclosedAbsent(t *testing.T) {
	t.Parallel()

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
		if _, ok := served[path]; ok {
			t.Errorf("served spec carries %s, want it disclosed-absent until the follow-up spec-port lane", path)
		}
	}
}
