// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

// D2 Leaf B Native-now termination (Refs eshu-hq/eshu#9669, part of
// #6707). This file owns the frozen wave table: the 18 routes across
// the 8 Native-now families the satellite terminates first for
// CoreReadContracts v0. It mirrors eshu's nativeNowWave
// (go/cmd/api/read_transport_conformance.go at the provenance pin)
// entry by entry; a wave change moves the eshu spec, the eshu skeleton
// table, and this mirror together. The behavior lives in
// leafb_terminate.go; the proofs in leafb_conformance_test.go and the
// per-family suites.
//
// Production binding (carried from the D0 shim proof): dest owns each
// route (caller gate, family authorizer, response mapping) and calls
// the versioned core read API as its port transport — bounded client,
// forwarded credential and negotiation headers, no retry, no
// table/queue handles dest-side. Authz stays atomic core-side: dest
// allows only the shared-key (AllScopes) identity it can construct and
// core re-judges every forwarded call as backstop (tenant scope,
// permission catalog, governance mode). Termination claims stay
// provisional until the arbiter re-reviews the D0 evidence (blocker
// B1); the contract record is contracts/leafb-native-now.md.

// Satellite serve kinds for the frozen wave. Wave-gated routes
// terminate for shared-key callers and relay explicitly for credential
// classes dest cannot resolve (scoped tokens, OIDC bearers, browser
// sessions — core judges, dest invents nothing). Wave-public routes
// are core-public pre-auth and carry no gate. Landed routes keep their
// existing serving and join the wave for frozenness only: openapi.json
// stays RequireAuth-gated by the standing no-new-public-surface
// decision, and /health stays the public probe.
const (
	leafbServeGated  = "wave-gated"
	leafbServePublic = "wave-public"
	leafbServeLanded = "landed"
)

// leafbRoute is one frozen Native-now wave entry: the exact cross-repo
// surface this lane terminates. Method+Path+Family mirror the eshu
// nativeNowWave entry; Serve tells the router how this lane serves it;
// Fragment names the Decision 5(c) fragment carrying the route's spec
// entry (or the mux-level marker for non-contract routes).
type leafbRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// leafbWave is the frozen first termination target: 18 routes across
// the 8 Native-now families (APIRouter, Capabilities, CLISupport, Ask,
// ServiceIntelHTTP, AuthProviders, SignInPolicyReads,
// AdminProviderConfigReads). TestLeafBWaveFrozen pins it entry by
// entry; a wave change moves the spec, the skeleton table, and this
// mirror together.
var leafbWave = []leafbRoute{
	{Method: "GET", Path: "/api/v0/openapi.json", Family: "api_router", Serve: leafbServeLanded, Fragment: "paths/status/routes.go"},
	{Method: "GET", Path: "/health", Family: "api_router", Serve: leafbServeLanded, Fragment: "<mux-health>"},
	{Method: "GET", Path: "/api/v0/docs", Family: "api_router", Serve: leafbServePublic, Fragment: "<doc-ui>"},
	{Method: "GET", Path: "/api/v0/redoc", Family: "api_router", Serve: leafbServePublic, Fragment: "<doc-ui>"},
	{Method: "GET", Path: "/api/v0/capabilities", Family: "capabilities", Serve: leafbServeGated, Fragment: "paths/catalog/capabilities.go"},
	{Method: "POST", Path: "/api/v0/answer-quality/score", Family: "cli_support", Serve: leafbServeGated, Fragment: "paths/cli/score.go"},
	{Method: "POST", Path: "/api/v0/investigation/packet/render", Family: "cli_support", Serve: leafbServeGated, Fragment: "paths/cli/render.go"},
	{Method: "POST", Path: "/api/v0/report/bundle/capture", Family: "cli_support", Serve: leafbServeGated, Fragment: "paths/cli/capture.go"},
	{Method: "POST", Path: "/api/v0/report/bundle/validate", Family: "cli_support", Serve: leafbServeGated, Fragment: "paths/cli/validate.go"},
	{Method: "POST", Path: "/api/v0/service/report/compose", Family: "cli_support", Serve: leafbServeGated, Fragment: "paths/cli/compose.go"},
	{Method: "POST", Path: "/api/v0/ask", Family: "ask", Serve: leafbServeGated, Fragment: "paths/search/ask.go"},
	{Method: "GET", Path: "/api/v0/services/{service_name}/intelligence-report", Family: "service_intel", Serve: leafbServeGated, Fragment: "paths/service/intelligence_report.go"},
	{Method: "GET", Path: "/api/v0/auth/providers", Family: "auth_providers", Serve: leafbServePublic, Fragment: "paths/auth/routes.go"},
	{Method: "GET", Path: "/api/v0/auth/sign-in-policy", Family: "sign_in_policy", Serve: leafbServePublic, Fragment: "paths/auth/sign_in_policy.go"},
	{Method: "GET", Path: "/api/v0/auth/admin/sign-in-policy", Family: "sign_in_policy", Serve: leafbServeGated, Fragment: "paths/auth/sign_in_policy.go"},
	{Method: "GET", Path: "/api/v0/auth/admin/provider-configs", Family: "provider_config", Serve: leafbServeGated, Fragment: "paths/auth/admin_provider_configs.go"},
	{Method: "GET", Path: "/api/v0/auth/admin/provider-configs/{provider_config_id}", Family: "provider_config", Serve: leafbServeGated, Fragment: "paths/auth/admin_provider_configs.go"},
	{Method: "GET", Path: "/api/v0/auth/admin/provider-configs/{provider_config_id}/revisions", Family: "provider_config", Serve: leafbServeGated, Fragment: "paths/auth/admin_provider_configs.go"},
}
