// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// W8a satellite termination (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#100, D13 tracker eshu-hq/eshu#10491). This file
// owns the frozen wave table: the 16 routes the seven deliberate
// pins cover. The behavior lives in w8a_terminate.go; the proofs in
// w8a_conformance_test.go, w8a_auth_test.go, the w8a parity suites,
// and w8a_latency_test.go; the spec posture in
// openapi/spec_w8a_test.go; the record in
// contracts/w8a-termination.md.
//
// Production binding (carried from the D0 shim proof via the D2-D6
// waves): dest owns the route (caller gate, family authorizer,
// response mapping) and calls the versioned core read API as its port
// transport — bounded client, forwarded credential and negotiation
// headers, no retry, no table/queue handles dest-side. Authz stays
// atomic core-side: dest allows only the shared-key (AllScopes)
// identity it can construct and core re-judges every forwarded call
// as backstop (tenant scope, permission catalog, governance mode).
//
// Transport pins (recorded byte-identical at the core HEAD
// eshu-hq/eshu#e504e328a7, re-verified in-lane with zero drift since
// each pin merge):
//   - SecretsIAMReadAdmission v0
//     (contracts/secretsiam-read-admission.md) covers the 5
//     secrets/IAM rows.
//   - StatusGovernanceReadAdmission v0
//     (contracts/status-governance-read-admission.md) covers the
//     governance row's Summary leg; the snapshot leg rides the
//     already-recorded StatusReadAdmission v0.
//   - StatusHostedReadinessReadAdmission v0
//     (contracts/status-hosted-readiness-read-admission.md) covers
//     the hosted-readiness row's graph legs; the snapshot leg rides
//     StatusReadAdmission v0.
//   - IaCReadAdmission v0 (contracts/iac-read-admission.md) covers
//     the 5 IaC rows.
//   - MetricsReadAdmission v0 (contracts/metrics-read-admission.md)
//     covers the metrics row.
//   - SemanticSearchReadAdmission v0
//     (contracts/semantic-search-read-admission.md) covers the
//     semantic row.
//   - TerraformDriftReadAdmission v0
//     (contracts/terraform-drift-read-admission.md) covers the
//     terraform row.
// The answer-narration row clears on StatusReadAdmission v0 alone:
// NarrationPosture is a handler func field, not a port, and rides
// core-side under the forward transport. The minting rule forbids
// inventing pins beyond these; the S3 external-principal grant leg
// stays OUT of the secrets pin and blends core-side on the forwarded
// call; the suppressions mutation stays out via SuppressionAdmission;
// Compare terminated in its gap-04 lane (see api/compare_wave.go).
//
// Scoping line (hard, from the pin contracts and the arbiter W8a
// ruling): recording a pin whole never authorizes terminating routes
// beyond these 16. POST /api/v0/cloud/runtime-drift/findings, GET
// /api/v0/iac/resources and every other IaC POST, the index-status
// Neo4j-count legs, the impact packet, and the code-graph-metrics
// POST stay relay with the reason disclosed per route in the
// contract; unused pin methods (the IaC Run/Summary/Reachability
// trio on these routes) stay unconsumed.
//
// The route stays inside RequireAuth: the wave gate below answers
// every credential shape (shared-key terminate, foreign-class relay,
// credential-less 401), so no keyless caller gains a new public
// surface here. There is no public carve-out in this lane — every
// W8a read is an authed reader-port read.

// W8a satellite serve kind. Every W8a route is wave-gated:
// shared-key callers terminate, unresolvable credential classes
// relay explicitly, credential-less callers (and junk-cookie
// carriers) keep the standing dest 401.
const w8aServeGated = "gated"

// w8aRoute is one frozen W8a wave entry: the exact cross-repo
// surface this lane terminates. Method+Path+Family mirror the
// CUTOVER rows; Serve tells the router how this lane serves it;
// Fragment names the core-side Decision 5(c) fragment the follow-up
// spec-port lane will vendor — this lane ports no fragment (the
// spec absences stay disclosed, see contracts/w8a-termination.md),
// so the field carries the core path, not a satellite one.
type w8aRoute struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// w8aWave is the frozen W8a termination target: the 16 routes behind
// the seven W8a pins. TestW8aWaveFrozen pins it entry by entry; a
// wave change moves the ownership map, the fragments, and this
// mirror together.
var w8aWave = []w8aRoute{
	{Method: "GET", Path: "/api/v0/secrets-iam/identity-trust-chains", Family: "secrets_iam", Serve: w8aServeGated, Fragment: "paths/infrastructure/secrets_iam.go"},
	{Method: "GET", Path: "/api/v0/secrets-iam/privilege-posture-observations", Family: "secrets_iam", Serve: w8aServeGated, Fragment: "paths/infrastructure/secrets_iam.go"},
	{Method: "GET", Path: "/api/v0/secrets-iam/secret-access-paths", Family: "secrets_iam", Serve: w8aServeGated, Fragment: "paths/infrastructure/secrets_iam.go"},
	{Method: "GET", Path: "/api/v0/secrets-iam/posture-gaps", Family: "secrets_iam", Serve: w8aServeGated, Fragment: "paths/infrastructure/secrets_iam.go"},
	{Method: "GET", Path: "/api/v0/secrets-iam/posture-summary", Family: "secrets_iam", Serve: w8aServeGated, Fragment: "paths/infrastructure/secrets_iam.go"},
	{Method: "GET", Path: "/api/v0/status/governance", Family: "status", Serve: w8aServeGated, Fragment: "paths/status/governance.go"},
	{Method: "GET", Path: "/api/v0/status/answer-narration", Family: "status", Serve: w8aServeGated, Fragment: "paths/status/answer_narration.go"},
	{Method: "GET", Path: "/api/v0/status/hosted-readiness", Family: "status", Serve: w8aServeGated, Fragment: "paths/status/hosted_readiness.go"},
	{Method: "POST", Path: "/api/v0/aws/runtime-drift/findings", Family: "iac", Serve: w8aServeGated, Fragment: "paths/cloud/aws_runtime_drift.go"},
	{Method: "GET", Path: "/api/v0/replatforming/selectors", Family: "iac", Serve: w8aServeGated, Fragment: "paths/iac/replatforming_selectors.go"},
	{Method: "POST", Path: "/api/v0/replatforming/rollups", Family: "iac", Serve: w8aServeGated, Fragment: "paths/iac/replatforming_rollups.go"},
	{Method: "POST", Path: "/api/v0/replatforming/plans", Family: "iac", Serve: w8aServeGated, Fragment: "paths/iac/replatforming.go"},
	{Method: "POST", Path: "/api/v0/replatforming/ownership-packets", Family: "iac", Serve: w8aServeGated, Fragment: "paths/iac/replatforming_ownership.go"},
	{Method: "GET", Path: "/api/v0/metrics/timeseries", Family: "metrics", Serve: w8aServeGated, Fragment: "paths/status/metrics.go"},
	{Method: "POST", Path: "/api/v0/search/semantic", Family: "semantic_search", Serve: w8aServeGated, Fragment: "paths/search/semantic.go"},
	{Method: "POST", Path: "/api/v0/terraform/config-state-drift/findings", Family: "terraform", Serve: w8aServeGated, Fragment: "paths/iac/terraform_config_state_drift.go"},
}

// registerW8aWave mounts the W8a termination wave ahead of the
// generic proxy: shared-key callers terminate, foreign classes relay
// explicitly, credential-less callers keep the standing 401, and
// each slash twin rides along never-split. Lives here (not in
// routes()) so the shared router stays under the file-size cap.
func (s *Server) registerW8aWave(mux *http.ServeMux) {
	for _, entry := range w8aWave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.w8aReadGate(http.HandlerFunc(s.serveW8aTerminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
