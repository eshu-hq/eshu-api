// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"sort"
	"strings"
	"testing"
)

// Own-scope spec proof (closes eshu-hq/eshu-api#65, part of
// eshu-hq/eshu#9669). The edge serves its own mounted scope at
// GET /api/v0/openapi.json from the same fragments that mount the
// routes, so the document cannot drift from the router: every
// natively mounted API operation has its spec entry, and every spec
// operation maps to a natively mounted route — modulo the disclosed
// absences below, each backed by its owning contract.
//
// The mounted set is rebuilt here from the exact tables and route
// constants the router registers (leafbWave, d3Wave, d4Wave, d6Wave,
// compareWave, codeFlowWave, codeGraphWave, divergenceWave, codeSearchWave, the
// per-family route lists, the mux-level probes),
// never from a second frozen copy: a lane change moves the router
// tables and this proof together. The spec side is read through the real route as a
// keyed caller, for both a configured and an unconfigured core
// origin, since the document serves with no core fetch.
//
// Forty-two mounted spellings stay out of the document by
// disclosure (eleven pre-W6a/W8a plus the 4 W6a Code-flow routes plus
// the 4 G-a Code-graph routes plus the 3 divergence routes plus the 2
// W4 Code search routes plus the 16 W8a routes plus the 2 drift routes),
// each with its owning contract: the three mux-level non-API mounts
// (/health, /readyz, /metrics), the two compiled doc-UI shells
// (/api/v0/docs, /api/v0/redoc), the one explicitly mounted relay leg
// whose fragment waits on its helper port (the repositories story
// GET, on the generic-relay stack in d4RelayLegs), the four D10
// setup-wizard routes whose fragment stays core-side by the recorded
// eshu-hq/eshu-api#44 posture outcome (contracts/setup-admission.md,
// pinned by TestSetupSpecPosture_StaysCoreSide), the gap-04 Compare
// POST whose fragment stays core-side for the relay-leg lane (core
// shapes it as its paths-closing entry, unportable mid-list —
// contracts/compare-termination.md, pinned by
// TestSpec_CompareStaysDisclosedAbsent), the four W6a Code-flow
// POSTs whose fragment stays core-side for the relay-leg lane (the
// four-key flow.go block ports entry-shaped in the follow-up
// spec-port lane — contracts/code-flow-termination.md, pinned by
// TestSpec_CodeFlowStaysDisclosedAbsent), the 2 W4 Code search POSTs
// whose fragments stay core-side for the relay-leg lane (the routes.go
// and symbols.go entries port entry-shaped with their referenced
// schemas in the follow-up spec-port lane —
// contracts/code-search-termination.md, pinned by
// TestSpec_CodeSearchStaysDisclosedAbsent), the 4 G-a Code-graph
// POSTs whose fragments stay core-side for the relay-leg lane (the
// routes.go, symbols.go, and route_to_caller.go entries port
// entry-shaped with their referenced schemas in the follow-up lane —
// contracts/code-graph-termination.md, pinned by
// openapi/spec_graph_test.go), the 3 divergence POSTs whose fragment
// stays core-side for the relay-leg lane (the divergence.go entries
// port entry-shaped with their referenced schemas in the follow-up
// lane — contracts/code-divergence-termination.md, pinned by
// openapi/spec_divergence_test.go), and the 16 W8a routes whose
// fragments stay core-side for the satellite termination lane
// (contracts/w8a-termination.md, pinned by openapi/spec_w8a_test.go).
// Only explicitly
// mounted routes are in scope: catch-all riders (the still-relayed
// core surface) stay out with the unbounded proxy itself. A newly
// mounted route fails here until its lane ports the fragment or
// discloses the absence; a newly served operation fails here until
// its lane mounts the route.

// specScopeAbsent pins the mounted spellings with no served spec
// entry, each with the contract that discloses it. A newly absent
// spelling fails the count pin until it lands here with its
// disclosure; a newly present one fails here until the owning lane
// removes it.
var specScopeAbsent = map[string]string{
	"GET /health":       "mux-health probe with a dest-owned shape, never an OpenAPI path (contracts/leafb-native-now.md)",
	"GET /readyz":       "liveness probe, public and unchanged (docs/skeleton.md)",
	"GET /metrics":      "Prometheus exposition, public like the probes (docs/telemetry.md)",
	"GET /api/v0/docs":  "compiled doc-UI shell, never an OpenAPI path (contracts/leafb-native-now.md)",
	"GET /api/v0/redoc": "compiled doc-UI shell, never an OpenAPI path (contracts/leafb-native-now.md)",
	"GET /api/v0/repositories/{repo_id}/story":               "explicitly mounted relay leg on the generic-relay stack (d4RelayLegs); fragment waits on the schema.EvidenceBoundaries helper port (contracts/d4-native-next.md, TestD4StoryRelayLegOnGenericProxy)",
	"GET /api/v0/auth/setup-state":                           "setup fragment stays core-side by the eshu-hq/eshu-api#44 posture outcome; the GET terminates regardless (contracts/setup-admission.md, TestSetupSpecPosture_StaysCoreSide)",
	"POST /api/v0/compare/environments":                      "compare fragment stays core-side for the relay-leg lane (core shapes it as its paths-closing entry); the POST terminates regardless (contracts/compare-termination.md, TestSpec_CompareStaysDisclosedAbsent)",
	"POST /api/v0/code/flow/cfg-summary":                     "code-flow fragment stays core-side for the relay-leg lane (four-key flow.go block ports entry-shaped in the follow-up lane); the POST terminates regardless (contracts/code-flow-termination.md, TestSpec_CodeFlowStaysDisclosedAbsent)",
	"POST /api/v0/code/flow/pdg-summary":                     "code-flow fragment stays core-side for the relay-leg lane (four-key flow.go block ports entry-shaped in the follow-up lane); the POST terminates regardless (contracts/code-flow-termination.md, TestSpec_CodeFlowStaysDisclosedAbsent)",
	"POST /api/v0/code/flow/reaching-def":                    "code-flow fragment stays core-side for the relay-leg lane (four-key flow.go block ports entry-shaped in the follow-up lane); the POST terminates regardless (contracts/code-flow-termination.md, TestSpec_CodeFlowStaysDisclosedAbsent)",
	"POST /api/v0/code/flow/taint-path":                      "code-flow fragment stays core-side for the relay-leg lane (four-key flow.go block ports entry-shaped in the follow-up lane); the POST terminates regardless (contracts/code-flow-termination.md, TestSpec_CodeFlowStaysDisclosedAbsent)",
	"POST /api/v0/code/call-chain":                           "code-graph fragment stays core-side for the relay-leg lane (the routes.go entries port entry-shaped with their referenced schemas in the follow-up lane); the POST terminates regardless (contracts/code-graph-termination.md, openapi/spec_graph_test.go)",
	"POST /api/v0/code/call-chain/compare":                   "code-graph fragment stays core-side for the relay-leg lane (the routes.go entries port entry-shaped with their referenced schemas in the follow-up lane); the POST terminates regardless (contracts/code-graph-termination.md, openapi/spec_graph_test.go)",
	"POST /api/v0/code/call-graph/metrics":                   "code-graph fragment stays core-side for the relay-leg lane (the symbols.go entry ports entry-shaped with its referenced schemas in the follow-up lane); the POST terminates regardless (contracts/code-graph-termination.md, openapi/spec_graph_test.go)",
	"POST /api/v0/code/routes/callers":                       "code-graph fragment stays core-side for the relay-leg lane (the route_to_caller.go entry ports entry-shaped with its referenced schemas in the follow-up lane); the POST terminates regardless (contracts/code-graph-termination.md, openapi/spec_graph_test.go)",
	"POST /api/v0/code/divergence/findings":                  "divergence fragment stays core-side for the relay-leg lane (the divergence.go entries port entry-shaped with their referenced schemas in the follow-up lane); the POST terminates regardless (contracts/code-divergence-termination.md, openapi/spec_divergence_test.go)",
	"POST /api/v0/code/divergence/investigate":               "divergence fragment stays core-side for the relay-leg lane (the divergence.go entries port entry-shaped with their referenced schemas in the follow-up lane); the POST terminates regardless (contracts/code-divergence-termination.md, openapi/spec_divergence_test.go)",
	"POST /api/v0/code/divergence/report":                    "divergence fragment stays core-side for the relay-leg lane (the divergence.go entries port entry-shaped with their referenced schemas in the follow-up lane); the POST terminates regardless (contracts/code-divergence-termination.md, openapi/spec_divergence_test.go)",
	"POST /api/v0/code/search":                               "code-search fragments stay core-side for the relay-leg lane (the routes.go and symbols.go entries port entry-shaped with their referenced schemas in the follow-up lane); the POST terminates regardless (contracts/code-search-termination.md, TestSpec_CodeSearchStaysDisclosedAbsent)",
	"POST /api/v0/code/symbols/search":                       "code-search fragments stay core-side for the relay-leg lane (the routes.go and symbols.go entries port entry-shaped with their referenced schemas in the follow-up lane); the POST terminates regardless (contracts/code-search-termination.md, TestSpec_CodeSearchStaysDisclosedAbsent)",
	"GET /api/v0/secrets-iam/identity-trust-chains":          "secrets-iam fragment stays core-side for the W8a lane; the GET terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"GET /api/v0/secrets-iam/privilege-posture-observations": "secrets-iam fragment stays core-side for the W8a lane; the GET terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"GET /api/v0/secrets-iam/secret-access-paths":            "secrets-iam fragment stays core-side for the W8a lane; the GET terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"GET /api/v0/secrets-iam/posture-gaps":                   "secrets-iam fragment stays core-side for the W8a lane; the GET terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"GET /api/v0/secrets-iam/posture-summary":                "secrets-iam fragment stays core-side for the W8a lane; the GET terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"GET /api/v0/status/governance":                          "status governance fragment stays core-side for the W8a lane; the GET terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"GET /api/v0/status/answer-narration":                    "status answer-narration fragment stays core-side for the W8a lane; the GET terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"GET /api/v0/status/hosted-readiness":                    "status hosted-readiness fragment stays core-side for the W8a lane; the GET terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"POST /api/v0/aws/runtime-drift/findings":                "cloud aws-runtime-drift fragment stays core-side for the W8a lane; the POST terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"GET /api/v0/replatforming/selectors":                    "iac selectors fragment stays core-side for the W8a lane; the GET terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"POST /api/v0/replatforming/rollups":                     "iac rollups fragment stays core-side for the W8a lane; the POST terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"POST /api/v0/replatforming/plans":                       "iac plans fragment stays core-side for the W8a lane; the POST terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"POST /api/v0/replatforming/ownership-packets":           "iac ownership-packets fragment stays core-side for the W8a lane; the POST terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"GET /api/v0/metrics/timeseries":                         "metrics fragment stays core-side for the W8a lane; the GET terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"POST /api/v0/search/semantic":                           "semantic fragment stays core-side for the W8a lane; the POST terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"POST /api/v0/terraform/config-state-drift/findings":     "terraform drift fragment stays core-side for the W8a lane; the POST terminates regardless (contracts/w8a-termination.md, openapi/spec_w8a_test.go)",
	"POST /api/v0/cloud/runtime-drift/findings":              "cloud runtime-drift fragment stays core-side for the drift lane; the POST terminates regardless (contracts/cloud-runtime-drift-termination.md, openapi/spec_drift_test.go)",
	"GET /api/v0/investigations/drift/packet":                "drift packet fragment stays core-side for the drift lane; the GET terminates regardless (contracts/cloud-runtime-drift-termination.md, openapi/spec_drift_test.go)",
	"POST /api/v0/auth/setup/claim":                          "setup fragment stays core-side by the eshu-hq/eshu-api#44 posture outcome; the POST rides tier-C dispatch regardless (contracts/setup-admission.md, TestSetupSpecPosture_StaysCoreSide)",
	"POST /api/v0/auth/setup/admin":                          "setup fragment stays core-side by the eshu-hq/eshu-api#44 posture outcome; the POST rides tier-C dispatch regardless (contracts/setup-admission.md, TestSetupSpecPosture_StaysCoreSide)",
	"POST /api/v0/auth/setup/mfa":                            "setup fragment stays core-side by the eshu-hq/eshu-api#44 posture outcome; the POST rides tier-C dispatch regardless (contracts/setup-admission.md, TestSetupSpecPosture_StaysCoreSide)",
}

// specScopeMounted rebuilds every explicitly mounted Method+Path the
// router registers, trailing-slash twins normalized away (each twin
// rides its exact spelling's handler for the same never-split
// reason). That includes the explicitly mounted D4 relay leg
// (d4RelayLegs): a bounded registration is in scope even though its
// bytes relay. The generic /api/ byte-relay catch-all is unbounded
// and stays out: catch-all riders are not listed anywhere, while
// every bounded registration is.
func specScopeMounted() []string {
	mounted := map[string]bool{}
	add := func(pattern string) {
		mounted[strings.TrimSuffix(pattern, "/")] = true
	}
	add("GET /health")
	add("GET /readyz")
	add("GET /metrics")
	add(suppressionRoute)
	add(signInPolicyRoute)
	add(retirementRoute)
	add(terminalFailureRoute)
	add(retireUnclaimedRoute)
	for _, route := range adminIdentityRoutes {
		add(route)
	}
	add(setupStateRoute)
	for _, route := range setupMutationRoutes {
		add(route)
	}
	for _, route := range providerConfigRoutes {
		add(route)
	}
	for _, route := range recoveryRoutes {
		add(route)
	}
	for _, route := range d7Routes {
		add(route)
	}
	for _, route := range d8GatedRoutes {
		add(route)
	}
	for _, route := range d8PublicRoutes {
		add(route)
	}
	for _, route := range d9Routes {
		add(route)
	}
	for _, route := range d9GitHubRoutes {
		add(route)
	}
	for _, route := range d9SAMLRoutes {
		add(route)
	}
	for _, entry := range leafbWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range d3Wave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range d3TrioWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range d4Wave {
		add(entry.Method + " " + entry.Path)
	}
	// The explicitly mounted D4 relay leg is in scope like every
	// other bounded registration (its absence carries the relay
	// disclosure below); only the unbounded catch-all stays out.
	for _, pattern := range d4RelayLegs {
		add(pattern)
	}
	for _, entry := range d6Wave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range freshnessWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range contentWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range contentSearchWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range compareWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range codeFlowWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range codeGraphWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range divergenceWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range codeSearchWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range w8aWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range driftWave {
		add(entry.Method + " " + entry.Path)
	}
	out := make([]string, 0, len(mounted))
	for key := range mounted {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// specScopeServed reads the served openapi.json exactly as a keyed
// caller would and returns its Method+Path operation set. Only real
// HTTP method entries count; path-item metadata never does.
func specScopeServed(t *testing.T, origin string) map[string]bool {
	t.Helper()

	paths := d5ServedPaths(t, origin)
	served := map[string]bool{}
	for path, raw := range paths {
		item, _ := raw.(map[string]any)
		for method := range item {
			switch method {
			case "get", "post", "patch", "delete", "put", "head", "options":
				served[strings.ToUpper(method)+" "+path] = true
			}
		}
	}
	return served
}

// TestSpecScope_Frozen pins the scope surface both sides enumerate:
// the mounted native count, the disclosed-absence count, and —
// implicitly, via the bijection below — the served operation count.
// A lane change (route mounted, renamed, or removed; absence
// disclosed or lifted) moves these pins with the owning lane.
//
// Arithmetic (trio + D6 + Wave-1 + Wave-2 + W4-search + Compare + W6a +
// W7-graph + divergence + W8a + drift + retire-unclaimed lanes): 167 + 3
// trio GETs + 3 D6 legs + 3 Wave-1 Freshness GETs + 3 Wave-2 Content
// POSTs + 4 W4 Content search POSTs + 2 W4 Code search POSTs + 1
// Compare POST + 4 Code-flow POSTs + 4 G-a Code-graph POSTs + 3
// divergence POSTs + 16 W8a reads + 2 drift reads + 1 retire-unclaimed
// POST = 216 mounted (215 native + 1 explicit relay leg); the
// disclosed-absence count is 42 (11 prior + 4 W6a Code-flow keys + 4
// G-a Code-graph keys + 3 divergence keys + 2 W4 Code search keys + 16
// W8a fragments + 2 drift fragments, each disclosed core-side by its
// owning lane; the W4 content-search fragment ports entry-shaped with
// the lane, and the retire-unclaimed fragment ports whole-file, so no
// new absence from those lanes).
func TestSpecScope_Frozen(t *testing.T) {
	t.Parallel()

	if got := len(specScopeMounted()); got != 216 {
		t.Fatalf("mounted operations = %d, want 216 (167 + 3 trio + 3 D6 + 3 Wave-1 + 3 Wave-2 + 4 W4 content-search + 2 W4 code-search + 1 Compare + 4 Code-flow + 4 G-a Code-graph + 3 divergence + 16 W8a + 2 drift + 1 retire-unclaimed = 215 native + 1 explicit relay leg): move the scope with the owning lane", got)
	}
	if got := len(specScopeAbsent); got != 42 {
		t.Fatalf("disclosed-absent operations = %d, want 42: each absence needs its disclosing contract", got)
	}
}

// TestSpecScope_MountedHasSpec pins the forward wiring: every mounted
// API operation (215 native + 1 explicit relay leg) has its served spec
// entry, for both a configured and an unconfigured core origin. Only
// the disclosed absences stay out.
func TestSpecScope_MountedHasSpec(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"http://core.invalid", ""} {
		served := specScopeServed(t, origin)
		for _, key := range specScopeMounted() {
			if _, absent := specScopeAbsent[key]; absent {
				continue
			}
			if !served[key] {
				t.Errorf("origin %q: mounted %s has no served spec entry: port the fragment or disclose the absence", origin, key)
			}
		}
	}
}

// TestSpecScope_SpecHasMount pins the reverse wiring: every served
// spec operation maps to a mounted route (native or explicit relay
// leg), for both a configured and an unconfigured core origin. The
// document can never promise what the edge does not mount.
func TestSpecScope_SpecHasMount(t *testing.T) {
	t.Parallel()

	mounted := map[string]bool{}
	for _, key := range specScopeMounted() {
		mounted[key] = true
	}
	for _, origin := range []string{"http://core.invalid", ""} {
		served := specScopeServed(t, origin)
		if got, want := len(served), len(mounted)-len(specScopeAbsent); got != want {
			t.Errorf("origin %q: served operations = %d, want %d (mounted minus disclosed absences)", origin, got, want)
		}
		for key := range served {
			if !mounted[key] {
				t.Errorf("origin %q: served %s maps to no mounted route: mount it or revert the fragment", origin, key)
			}
		}
		for key := range specScopeAbsent {
			if served[key] {
				t.Errorf("origin %q: served operations gained disclosed-absent %s: claim it in the mounted set", origin, key)
			}
		}
	}
}

// TestSpecScope_AbsentMatchesMounted pins that every disclosed
// absence matches a mounted spelling: no stale disclosures survive
// their lane.
func TestSpecScope_AbsentMatchesMounted(t *testing.T) {
	t.Parallel()

	mounted := map[string]bool{}
	for _, key := range specScopeMounted() {
		mounted[key] = true
	}
	for key := range specScopeAbsent {
		if !mounted[key] {
			t.Errorf("disclosed-absent %s matches no mounted route: drop it with the owning lane", key)
		}
	}
}
