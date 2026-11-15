// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// W8b-narrow relay-leg termination spec posture (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#101). The 15
// narrow-family routes mount natively in the W8b-narrow wave lane,
// but their Decision 5(c) fragments stay core-side for this lane:
// each entry lives in its owning core paths file (see
// api/w8bnarrow_wave.go Fragment column), and porting any entry
// without its referenced blocks would break the verbatim-port rule
// scripts/check-parity.sh gates — so, like the W8a, drift, and G
// lanes, the routes are disclosed spec absences (see
// contracts/w8b-narrow-termination.md), not a port. A follow-up
// spec-port lane claims the keys when it ports the entries
// entry-shaped with their referenced schemas; until then no served
// key duplicates a relayed route, and no mounted route promises what
// the document does not carry.

// TestSpec_W8bNarrowStaysDisclosedAbsent pins the W8b-narrow spec
// posture: the served document carries none of the 15 narrow keys
// while the fragments stay core-side. Lifting any absence (porting
// the entries) belongs to the follow-up spec-port lane, not to the
// relay-leg termination.
func TestSpec_W8bNarrowStaysDisclosedAbsent(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())

	for _, path := range []string{
		"/api/v0/kubernetes/correlations",
		"/api/v0/observability/coverage/correlations",
		"/api/v0/work-items/evidence",
		"/api/v0/semantic/code-hints",
		"/api/v0/semantic/documentation-observations",
		"/api/v0/dependencies",
		"/api/v0/graph/entities",
		"/api/v0/images",
		"/api/v0/images/tag-history",
		"/api/v0/incidents/{incident_id}/context",
		"/api/v0/runtime-declarations/query",
		"/api/v0/cloud/inventory",
		"/api/v0/cloud/ssm-host-inventory",
		"/api/v0/service-catalog/correlations",
		"/api/v0/codeowners/ownership",
	} {
		if _, ok := served[path]; ok {
			t.Errorf("served spec carries %s, want it disclosed-absent until the follow-up spec-port lane", path)
		}
	}
}
