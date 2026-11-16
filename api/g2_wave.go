// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// G2 reads termination (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#101 RE:G2V2). This file owns the frozen wave
// table: exactly the 7 G2 GETs and POST the 5 core read-admission v0
// pins admit (core eshu-hq/eshu#10796, merged 64de9fc7f1): the 2
// ExtractionReadiness reads behind ExtractionReadiness read-admission
// v0, the 2 FactSchemaVersions reads behind FactSchema read-admission
// v0, the SurfaceInventory read behind SurfaceInventory
// read-admission v0, the visualizations derive POST behind
// Visualization read-admission v0, and the manifest-dependencies read
// behind ManifestDependencies read-admission v0. A wave change moves
// the ownership map, the spec fragments, and this mirror together.
// The behavior lives in g2_terminate.go; the proofs in
// g2_conformance_test.go and the per-family suites.
//
// Production binding (carried from the Wave-1b lane via the D0 shim
// proof): dest owns each route (caller gate, family authorizer,
// response mapping) and calls the versioned core read API as its port
// transport — the shared-key credential and negotiation headers
// forwarded, no retry, no table/queue handles dest-side. Authz stays
// atomic core-side: dest allows only the shared-key (AllScopes)
// identity it can construct and core re-judges every forwarded call
// as backstop (tenant scope, permission catalog, governance mode —
// which is also where the live 403s and mode-dependent denials come
// from). The contract record is contracts/g2-wave.md.
//
// Transport pins: ExtractionReadiness read-admission v0,
// FactSchema read-admission v0, SurfaceInventory read-admission v0,
// Visualization read-admission v0, and ManifestDependencies
// read-admission v0 (recorded at the core merge 64de9fc7f1) cover all
// 7 wave rows — every row admits through its re-declared port method
// over the guarded read port. The minting rule forbids inventing pins
// beyond these 5; this lane cites no other pin, and no other family
// may cite these pins.
//
// D-wave satellite serve kind. Every G2 route is wave-gated:
// shared-key callers terminate, unresolvable credential classes relay
// explicitly, credential-less callers (and junk-cookie carriers, per
// the D2 follow-up lesson) keep the standing 401. There is no public
// carve-out in this lane — every G2 read is an authed reader-port
// read. The routes stay inside RequireAuth: the wave gate answers
// every credential shape, so no keyless caller gains a new public
// surface here.
//
// Every leg classifies to the shared "other" family in
// classifyFamily (no per-family prefix entries exist for these
// paths): the authorizer and telemetry pin that value until family
// entries land, exactly like the Wave-1b component-extension legs.
// The CUTOVER families ride the row comments below and
// contracts/g2-wave.md, not the table.

// g2ServeGated is the only serve kind in this lane.
const g2ServeGated = "gated"

// G2 wave route paths: the exact core route paths the pins admit.
// The wave table references these so the frozen test, the router
// registration, and the terminated-handler switch move together.
const (
	g2CollectorExtractionReadinessPath    = "/api/v0/collector-extraction-readiness"
	g2CollectorExtractionReadinessFamPath = "/api/v0/collector-extraction-readiness/{family}"
	g2FactSchemaVersionsPath              = "/api/v0/fact-schema-versions"
	g2FactSchemaVersionKindPath           = "/api/v0/fact-schema-versions/{fact_kind}"
	g2SurfaceInventoryPath                = "/api/v0/surface-inventory"
	g2VisualizationsDerivePath            = "/api/v0/visualizations/derive"
	g2RepositoryManifestDependenciesPath  = "/api/v0/repository/manifest-dependencies"
)

// g2Route is one frozen G2 entry: the exact cross-repo surface this
// lane terminates. Method+Path+Family mirror the CUTOVER rows (the
// CUTOVER family rides the row comment; Family carries the satellite
// serve family classifyFamily reports); Serve tells the router how
// this lane serves it; Fragment names the Decision 5(c) core
// fragment that carries the route's spec entry (the core-side path —
// this lane ports no fragment, the spec absences stay disclosed, see
// contracts/g2-wave.md).
type g2Route struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// g2Wave is the frozen G2 termination target: exactly the 7 reads
// behind the 5 core read-admission v0 pins.
// TestG2WaveFrozen pins it entry by entry; a wave change moves the
// ownership map, the fragments, and this mirror together.
var g2Wave = []g2Route{
	// CUTOVER family ExtractionReadiness.
	{Method: "GET", Path: g2CollectorExtractionReadinessPath, Family: "other", Serve: g2ServeGated, Fragment: "paths/status/collector_extraction_readiness.go"},
	{Method: "GET", Path: g2CollectorExtractionReadinessFamPath, Family: "other", Serve: g2ServeGated, Fragment: "paths/status/collector_extraction_readiness.go"},
	// CUTOVER family FactSchemaVersions.
	{Method: "GET", Path: g2FactSchemaVersionsPath, Family: "other", Serve: g2ServeGated, Fragment: "paths/status/fact_schema_version.go"},
	{Method: "GET", Path: g2FactSchemaVersionKindPath, Family: "other", Serve: g2ServeGated, Fragment: "paths/status/fact_schema_version.go"},
	// CUTOVER family SurfaceInventory.
	{Method: "GET", Path: g2SurfaceInventoryPath, Family: "other", Serve: g2ServeGated, Fragment: "paths/catalog/surface_inventory.go"},
	// CUTOVER family Visualization.
	{Method: "POST", Path: g2VisualizationsDerivePath, Family: "other", Serve: g2ServeGated, Fragment: "paths/evidence/visualization_packets.go"},
	// CUTOVER family ManifestDependencies.
	{Method: "GET", Path: g2RepositoryManifestDependenciesPath, Family: "other", Serve: g2ServeGated, Fragment: "paths/repository/manifest_dependencies.go"},
}

// registerG2Wave mounts the G2 reads termination (Refs
// eshu-hq/eshu#9669): the 7 pinned read routes register explicitly
// inside the wave gate, ahead of the generic proxy. A valid shared
// key terminates on the native lane; a foreign credential class
// (scoped token, OIDC bearer, session cookie) relays explicitly on
// the relay lane so core judges it; credential-less callers — and
// junk-cookie carriers, per the D2 follow-up lesson — get the
// standing 401. Each trailing-slash spelling stays on the wave route
// for the same never-split reason. Every other family still crosses
// its existing lane.
func (s *Server) registerG2Wave(mux *http.ServeMux) {
	for _, entry := range g2Wave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.g2ReadGate(http.HandlerFunc(s.g2TerminatedHandler(entry.Path))))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
