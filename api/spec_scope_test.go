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
// operation maps to a natively mounted route — modulo the five
// non-API mounts below, which have no spec entry by design.
//
// The mounted set is rebuilt here from the exact tables and route
// constants the router registers (leafbWave, d3Wave, d3TrioWave, the
// adminTuningReportRoute singleton,
// d4Wave, d6Wave, compareWave, codeFlowWave, codeGraphWave,
// deadCodeWave, codeDivergenceWave, relationshipWave, codeScanWave,
// codeInvestigationWave, codeSecretsWave, codeCypherWave,
// codeLanguageWave, codeSearchWave, w8aWave, driftWave, w8bNarrowWave,
// entityWave, impactWave, storyWave, packetWave, g2Wave, compextWave,
// iacNativeRoutes, iacResourcesRoutes, the per-family route lists, the
// mux-level probes), never from a second frozen copy: a lane change
// moves the router tables and this proof together. The spec side is
// read through the real route as a keyed caller, for both a configured
// and an unconfigured core origin, since the document serves with no
// core fetch.
//
// Five mounted spellings stay out of the document by design (the
// three mux-level non-API mounts (/health, /readyz, /metrics) and the
// two compiled doc-UI shells (/api/v0/docs, /api/v0/redoc)): probes
// and doc browsers are not API operations and never gain path keys.
// The D13 row-7 full-surface lane closed every disclosed API absence,
// so the document covers the full CUTOVER route surface: every mounted
// API operation has its served spec entry.
//
// Only explicitly
// mounted routes are in scope: catch-all riders (the unmounted core
// remainder — e.g. unmounted methods such as PATCH
// /api/v0/code/bundles — still cross the generic proxy) stay out
// with the unbounded proxy itself. A newly
// mounted route fails here until its lane ports the fragment or
// discloses the absence; a newly served operation fails here until
// its lane mounts the route.

// specScopeAbsent pins the mounted spellings with no served spec
// entry. Only the by-design non-API mounts remain: a newly absent API
// spelling fails the count pin until it lands here with its
// disclosure; a newly present one fails here until the owning lane
// removes it.
var specScopeAbsent = map[string]string{
	"GET /health":       "mux-health probe with a dest-owned shape, never an OpenAPI path (contracts/leafb-native-now.md)",
	"GET /readyz":       "liveness probe, public and unchanged (docs/skeleton.md)",
	"GET /metrics":      "Prometheus exposition, public like the probes (docs/telemetry.md)",
	"GET /api/v0/docs":  "compiled doc-UI shell, never an OpenAPI path (contracts/leafb-native-now.md)",
	"GET /api/v0/redoc": "compiled doc-UI shell, never an OpenAPI path (contracts/leafb-native-now.md)",
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
	// The promoted tuning-report singleton registers on its own anchor
	// ahead of the D3 loops (never-split twin normalized away by add).
	add(adminTuningReportRoute)
	// The promoted work-items query singleton registers on its own
	// anchor ahead of the D3 loops (likewise normalized).
	add(adminWorkItemsRoute)
	// The promoted dead-letters query singleton registers on its own
	// anchor ahead of the D3 loops (likewise normalized).
	add(adminDeadLettersRoute)
	// The promoted decisions query singleton registers on its own
	// anchor ahead of the D3 loops (likewise normalized).
	add(adminDecisionsRoute)
	// The promoted reindex-progress singleton registers on its own
	// anchor ahead of the D3 loops (likewise normalized).
	add(adminReindexProgressRoute)
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
	for _, entry := range deadCodeWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range codeDivergenceWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range relationshipWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range codeScanWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range codeInvestigationWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range codeSecretsWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range codeCypherWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range codeLanguageWave {
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
	for _, entry := range w8bNarrowWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range entityWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range impactWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range storyWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range packetWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range g2Wave {
		add(entry.Method + " " + entry.Path)
	}
	for _, entry := range compextWave {
		add(entry.Method + " " + entry.Path)
	}
	for _, route := range iacNativeRoutes {
		add(route)
	}
	for _, route := range iacResourcesRoutes {
		add(route)
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
// Arithmetic: the 269 previously mounted operations plus the lanes
// this proof now enumerates (7 G2 reads + 2 Wave-1b
// component-extension reads + 6 W3 IaC reads) = 284 mounted (284
// native + 0 explicit relay holds); the by-design exclusion count is 5
// (the three mux-level non-API mounts plus the two doc-UI shells), so
// the served operation count reconciles at 279: the full CUTOVER route
// surface.
func TestSpecScope_Frozen(t *testing.T) {
	t.Parallel()

	if got := len(specScopeMounted()); got != 284 {
		t.Fatalf("mounted operations = %d, want 284 (269 + 7 G2 + 2 Wave-1b + 6 W3 IaC): move the scope with the owning lane", got)
	}
	if got := len(specScopeAbsent); got != 5 {
		t.Fatalf("non-API exclusions = %d, want 5 (health, readyz, metrics, docs, redoc): each exclusion needs its by-design rationale", got)
	}
}

// TestSpecScope_MountedHasSpec pins the forward wiring: every mounted
// API operation (284 native + 0 explicit relay holds) has its served
// spec entry, for both a configured and an unconfigured core origin.
// Only the by-design non-API exclusions stay out.
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
			t.Errorf("origin %q: served operations = %d, want %d (mounted minus by-design exclusions)", origin, got, want)
		}
		for key := range served {
			if !mounted[key] {
				t.Errorf("origin %q: served %s maps to no mounted route: mount it or revert the fragment", origin, key)
			}
		}
		for key := range specScopeAbsent {
			if served[key] {
				t.Errorf("origin %q: served operations gained excluded non-API %s: it has no spec entry by design", origin, key)
			}
		}
	}
}

// TestSpecScope_AbsentMatchesMounted pins that every by-design
// exclusion matches a mounted spelling: no stale exclusions survive
// their lane.
func TestSpecScope_AbsentMatchesMounted(t *testing.T) {
	t.Parallel()

	mounted := map[string]bool{}
	for _, key := range specScopeMounted() {
		mounted[key] = true
	}
	for key := range specScopeAbsent {
		if !mounted[key] {
			t.Errorf("excluded %s matches no mounted route: drop it with the owning lane", key)
		}
	}
}
