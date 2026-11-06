// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

// D4 Native-next reads termination (Refs eshu-hq/eshu#9669, part of
// #6707). This file owns the frozen wave table: the 46 routes across
// the 8 Native-next families the satellite terminates after the D3
// admin/identity/recovery reads lane. It mirrors the CUTOVER_OWNERSHIP
// rows for CICD, Documentation, Infra, PackageRegistry, Repositories,
// Evidence, InvestigationWorkflows, and Playbooks (Compare deferred to
// D6); a wave change moves the ownership map, the spec fragments, and
// this mirror together. The behavior lives in d4_terminate.go; the
// proofs in d4_conformance_test.go and the per-family suites.
//
// Production binding (carried from the D0 shim proof via the D2/D3
// waves): dest owns each route (caller gate, family authorizer,
// response mapping) and calls the versioned core read API as its port
// transport — bounded client, forwarded credential and negotiation
// headers, no retry, no table/queue handles dest-side. Authz stays
// atomic core-side: dest allows only the shared-key (AllScopes)
// identity it can construct and core re-judges every forwarded call as
// backstop (tenant scope, permission catalog, governance mode).
// Termination claims stay provisional until the arbiter re-reviews the
// D0 evidence (blocker B1); the contract record is
// contracts/d4-native-next.md.
//
// Transport pin: CoreReadContractsVersion v0 (contracts/upstream/
// api_boundary.go at the provenance pin) covers every D4 family — all
// 46 rows are `read`-channel rows on reader ports only
// (db.ReadStore, graph GraphQuery, ContentStore) per the ownership map,
// so no per-family admission alias is minted or needed here. The
// minting rule forbids inventing one.

// D4 satellite serve kind. Every D4 route is wave-gated: shared-key
// callers terminate, unresolvable credential classes relay explicitly,
// credential-less callers (and junk-cookie carriers) keep the standing
// dest 401. There is no public carve-out in this lane — every D4 read
// is an authed reader-port read.
const d4ServeGated = "gated"

// d4SlashSkip holds the wave paths whose trailing-slash spelling must
// NOT double-register on the native route. Go 1.22+ ServeMux panics
// when a literal trailing-slash subtree (e.g.
// "GET /api/v0/documentation/findings/count/") overlaps a wildcard
// sibling (e.g.
// "GET /api/v0/documentation/findings/{finding_id}/evidence-packet")
// without subsuming it — unlike the list-slash + wildcard-child pairs
// the earlier lanes register, which nest cleanly. The four entries
// below are exactly that shape: the skip avoids the duplicate
// registration panic, and the slash spellings still terminate natively
// under the parent list-subtree registration (findings/, repositories/)
// — still the never-split rule, still core bytes. TestD4SlashSkipPinned
// pins the set; TestD4ConflictLiterals_TrailingSlashStaysNative pins
// the parent-subtree serving.
var d4SlashSkip = map[string]bool{
	"GET /api/v0/documentation/findings/count":     true,
	"GET /api/v0/documentation/findings/inventory": true,
	"GET /api/v0/repositories/by-language":         true,
	"GET /api/v0/repositories/language-inventory":  true,
}

// d4Route is one frozen D4 wave entry: the exact cross-repo surface
// this lane terminates. Method+Path+Family mirror the CUTOVER rows;
// Serve tells the router how this lane serves it; Fragment names the
// Decision 5(c) fragment that carries the route's spec entry.
type d4Route struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// d4Wave is the frozen D4 termination target: 46 routes across the 8
// Native-next families (CICD, Documentation, Infra, PackageRegistry,
// Repositories, Evidence, InvestigationWorkflows, Playbooks).
// TestD4WaveFrozen pins it entry by entry; a wave change moves the
// ownership map, the fragments, and this mirror together.
var d4Wave = []d4Route{
	{Method: "GET", Path: "/api/v0/catalog", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/routes.go"},
	{Method: "GET", Path: "/api/v0/ci-cd/run-correlations", Family: "cicd", Serve: d4ServeGated, Fragment: "paths/cicd/routes.go"},
	{Method: "GET", Path: "/api/v0/ci-cd/run-correlations/count", Family: "cicd", Serve: d4ServeGated, Fragment: "paths/cicd/run_correlation_aggregate.go"},
	{Method: "GET", Path: "/api/v0/ci-cd/run-correlations/inventory", Family: "cicd", Serve: d4ServeGated, Fragment: "paths/cicd/run_correlation_aggregate.go"},
	{Method: "GET", Path: "/api/v0/cloud/resources", Family: "infra", Serve: d4ServeGated, Fragment: "paths/cloud/routes.go"},
	{Method: "GET", Path: "/api/v0/documentation/evidence-packets/{packet_id}/freshness", Family: "documentation", Serve: d4ServeGated, Fragment: "paths/evidence/documentation.go"},
	{Method: "GET", Path: "/api/v0/documentation/facts", Family: "documentation", Serve: d4ServeGated, Fragment: "paths/evidence/documentation.go"},
	{Method: "GET", Path: "/api/v0/documentation/findings", Family: "documentation", Serve: d4ServeGated, Fragment: "paths/evidence/documentation.go"},
	{Method: "GET", Path: "/api/v0/documentation/findings/count", Family: "documentation", Serve: d4ServeGated, Fragment: "paths/evidence/documentation_finding_aggregate.go"},
	{Method: "GET", Path: "/api/v0/documentation/findings/inventory", Family: "documentation", Serve: d4ServeGated, Fragment: "paths/evidence/documentation_finding_aggregate.go"},
	{Method: "GET", Path: "/api/v0/documentation/findings/{finding_id}/evidence-packet", Family: "documentation", Serve: d4ServeGated, Fragment: "paths/evidence/documentation.go"},
	{Method: "GET", Path: "/api/v0/ecosystem/overview", Family: "infra", Serve: d4ServeGated, Fragment: "paths/infrastructure/routes.go"},
	{Method: "GET", Path: "/api/v0/evidence/admission-decisions", Family: "evidence", Serve: d4ServeGated, Fragment: "paths/evidence/routes.go"},
	{Method: "GET", Path: "/api/v0/evidence/bundle", Family: "evidence", Serve: d4ServeGated, Fragment: "paths/evidence/bundle.go"},
	{Method: "GET", Path: "/api/v0/evidence/relationships/{resolved_id}", Family: "evidence", Serve: d4ServeGated, Fragment: "paths/evidence/routes.go"},
	{Method: "GET", Path: "/api/v0/infra/resources/count", Family: "infra", Serve: d4ServeGated, Fragment: "paths/infrastructure/resource_aggregate.go"},
	{Method: "GET", Path: "/api/v0/infra/resources/inventory", Family: "infra", Serve: d4ServeGated, Fragment: "paths/infrastructure/resource_aggregate.go"},
	{Method: "GET", Path: "/api/v0/investigation-workflows", Family: "investigation_workflows", Serve: d4ServeGated, Fragment: "paths/evidence/investigation_workflows.go"},
	{Method: "GET", Path: "/api/v0/investigations/deployable-unit/packet", Family: "evidence", Serve: d4ServeGated, Fragment: "paths/evidence/investigations.go"},
	{Method: "GET", Path: "/api/v0/package-registry/correlations", Family: "package_registry", Serve: d4ServeGated, Fragment: "paths/repository/package_registry.go"},
	{Method: "GET", Path: "/api/v0/package-registry/dependencies", Family: "package_registry", Serve: d4ServeGated, Fragment: "paths/repository/package_registry.go"},
	{Method: "GET", Path: "/api/v0/package-registry/dependency-chains", Family: "package_registry", Serve: d4ServeGated, Fragment: "paths/repository/package_registry.go"},
	{Method: "GET", Path: "/api/v0/package-registry/packages", Family: "package_registry", Serve: d4ServeGated, Fragment: "paths/repository/package_registry.go"},
	{Method: "GET", Path: "/api/v0/package-registry/packages/count", Family: "package_registry", Serve: d4ServeGated, Fragment: "paths/repository/package_registry_aggregate.go"},
	{Method: "GET", Path: "/api/v0/package-registry/packages/inventory", Family: "package_registry", Serve: d4ServeGated, Fragment: "paths/repository/package_registry_aggregate.go"},
	{Method: "GET", Path: "/api/v0/package-registry/versions", Family: "package_registry", Serve: d4ServeGated, Fragment: "paths/repository/package_registry.go"},
	{Method: "GET", Path: "/api/v0/query-playbooks", Family: "playbooks", Serve: d4ServeGated, Fragment: "paths/catalog/playbooks.go"},
	{Method: "GET", Path: "/api/v0/repositories", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/routes.go"},
	{Method: "GET", Path: "/api/v0/repositories/by-language", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/routes.go"},
	{Method: "GET", Path: "/api/v0/repositories/language-inventory", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/routes.go"},
	{Method: "GET", Path: "/api/v0/repositories/{repo_id}/branches", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/branches.go"},
	{Method: "GET", Path: "/api/v0/repositories/{repo_id}/content", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/routes.go"},
	{Method: "GET", Path: "/api/v0/repositories/{repo_id}/context", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/routes.go"},
	{Method: "GET", Path: "/api/v0/repositories/{repo_id}/coverage", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/stats_and_coverage.go"},
	{Method: "GET", Path: "/api/v0/repositories/{repo_id}/freshness", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/freshness.go"},
	{Method: "GET", Path: "/api/v0/repositories/{repo_id}/stats", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/stats_and_coverage.go"},
	{Method: "GET", Path: "/api/v0/repositories/{repo_id}/story", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/routes.go"},
	{Method: "GET", Path: "/api/v0/repositories/{repo_id}/tree", Family: "repositories", Serve: d4ServeGated, Fragment: "paths/repository/routes.go"},
	{Method: "POST", Path: "/api/v0/ecosystem/graph-summary", Family: "infra", Serve: d4ServeGated, Fragment: "paths/infrastructure/routes.go"},
	{Method: "POST", Path: "/api/v0/evidence/citations", Family: "evidence", Serve: d4ServeGated, Fragment: "paths/evidence/routes.go"},
	{Method: "POST", Path: "/api/v0/infra/relationships", Family: "infra", Serve: d4ServeGated, Fragment: "paths/infrastructure/routes.go"},
	{Method: "POST", Path: "/api/v0/infra/resources/search", Family: "infra", Serve: d4ServeGated, Fragment: "paths/infrastructure/routes.go"},
	{Method: "POST", Path: "/api/v0/investigation-workflows/resolve", Family: "investigation_workflows", Serve: d4ServeGated, Fragment: "paths/evidence/investigation_workflows.go"},
	{Method: "POST", Path: "/api/v0/query-playbooks/resolve", Family: "playbooks", Serve: d4ServeGated, Fragment: "paths/catalog/playbooks.go"},
	{Method: "POST", Path: "/api/v0/relationships/catalog", Family: "infra", Serve: d4ServeGated, Fragment: "paths/infrastructure/routes.go"},
	{Method: "POST", Path: "/api/v0/relationships/edges", Family: "infra", Serve: d4ServeGated, Fragment: "paths/infrastructure/routes.go"},
}
