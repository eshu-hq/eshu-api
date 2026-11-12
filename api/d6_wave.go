// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// D6 Status + SupplyChain reads termination (Refs eshu-hq/eshu#9669,
// part of #6707). This file owns the frozen wave table: the 32 routes
// across the Status and SupplyChain families the satellite terminates
// after the D4 Native-next lane. It mirrors the CUTOVER_OWNERSHIP rows
// for the Status reads admitting through StatusReadAdmission v0
// (status.Reader) and the SupplyChain reads admitting through
// SupplyChainReadAdmission v0 (the 17 family-owned reader ports),
// plus the phase-2 extra-port Status legs admitting through
// StatusExtraReadAdmission v0 (the bounded LiveActivity read and the
// retirement-summary read); a wave change moves the ownership map,
// the spec fragments, and this mirror together. The behavior lives in
// d6_terminate.go; the proofs in d6_conformance_test.go and the
// per-family suites.
//
// Production binding (carried from the D0 shim proof via the D2/D4
// waves): dest owns each route (caller gate, family authorizer,
// response mapping) and calls the versioned core read API as its port
// transport — bounded client, forwarded credential and negotiation
// headers, no retry, no table/queue handles dest-side. Authz stays
// atomic core-side: dest allows only the shared-key (AllScopes)
// identity it can construct and core re-judges every forwarded call as
// backstop (tenant scope, permission catalog, governance mode).
// Termination claims stay provisional until the arbiter re-reviews the
// D0 evidence (blocker B1); the contract record is
// contracts/d6-status-supplychain.md.
//
// Transport pins: StatusReadAdmission v0 (contracts/upstream/
// status_read_admission_contract_test.go at the D6 pin) covers the 12
// Status wave rows — every row's snapshot read admits through
// status.Reader (ReadStatusSnapshot, ReadStatusSnapshotFiltered).
// SupplyChainReadAdmission v0 (contracts/upstream/
// supply_read_admission_contract_test.go at the D6 pin) covers the 17
// SupplyChain wave rows — every row admits through the frozen
// family-owned reader ports. StatusExtraReadAdmission v0
// (contracts/upstream/status_extra_read_admission_contract_test.go at
// the phase-2 pin) covers the 3 extra-port Status legs — the
// live-activity leg on operations and the retirements leg on both
// index spellings. The minting rule forbids inventing pins beyond
// these three; the suppressions mutation stays out via
// SuppressionAdmission, and Compare carries its own deliberate later
// pin in its gap-04 lane (CompareReadAdmission v0 — see
// api/compare_wave.go).
//
// Scoping line (hard, from the pin contracts): the Status pin never
// covers the GovernanceAudit Summary, the NarrationPosture override,
// or the Neo4j index counts — governance, answer-narration, and
// hosted-readiness stay relay with the reason disclosed per route in
// the contract. The terminated index and operations routes still cross
// unpinned legs (Neo4j counts, the narration override, env-derived
// config); those legs ride the terminated forward as core-served
// bytes with no dest-owned port, disclosed per route in the contract.
// The investigations impact packet admits through the writer
// ImpactPacketResponder plus shared graph/content ports the pin
// explicitly excludes, so it stays relay. Compare
// (POST /api/v0/compare/environments) admitted only through the
// shared ports until its deliberate later pin landed
// (CompareReadAdmission v0, eshu-hq/eshu#10425); it terminates in
// its own gap-04 lane now (see api/compare_wave.go).
//
// D6 satellite serve kind. Every D6 route is wave-gated: shared-key
// callers terminate, unresolvable credential classes relay explicitly,
// credential-less callers (and junk-cookie carriers) keep the standing
// dest 401. There is no public carve-out in this lane — every D6 read
// is an authed reader-port read.
const d6ServeGated = "gated"

// d6Route is one frozen D6 wave entry: the exact cross-repo surface
// this lane terminates. Method+Path+Family mirror the CUTOVER rows;
// Serve tells the router how this lane serves it; Fragment names the
// Decision 5(c) fragment that carries the route's spec entry.
type d6Route struct {
	Method   string
	Path     string
	Family   string
	Serve    string
	Fragment string
}

// d6Wave is the frozen D6 termination target: 12 Status routes plus 17
// SupplyChain routes, plus the 3 phase-2 extra-port Status legs behind
// StatusExtraReadAdmission v0. TestD6WaveFrozen pins it entry by
// entry; a wave change moves the ownership map, the fragments, and
// this mirror together.
var d6Wave = []d6Route{
	{Method: "GET", Path: "/api/v0/collector-readiness", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/collector_readiness.go"},
	{Method: "GET", Path: "/api/v0/collectors", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/status_reads.go"},
	{Method: "GET", Path: "/api/v0/index-status", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/status_extra_reads.go"},
	{Method: "GET", Path: "/api/v0/ingesters", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/status_reads.go"},
	{Method: "GET", Path: "/api/v0/ingesters/{ingester}", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/status_reads.go"},
	{Method: "GET", Path: "/api/v0/status/collector-readiness", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/collector_readiness.go"},
	{Method: "GET", Path: "/api/v0/status/collectors", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/status_reads.go"},
	{Method: "GET", Path: "/api/v0/status/freshness-causality", Family: "status", Serve: d6ServeGated, Fragment: "paths/freshness/causality.go"},
	{Method: "GET", Path: "/api/v0/status/index", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/status_extra_reads.go"},
	{Method: "GET", Path: "/api/v0/status/ingesters", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/status_reads.go"},
	{Method: "GET", Path: "/api/v0/status/ingesters/{ingester}", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/status_reads.go"},
	{Method: "GET", Path: "/api/v0/status/operations", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/operations.go"},
	{Method: "GET", Path: "/api/v0/status/operator-control-plane", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/operator_control_plane.go"},
	{Method: "GET", Path: "/api/v0/status/pipeline", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/status_reads.go"},
	{Method: "GET", Path: "/api/v0/status/semantic-extraction", Family: "status", Serve: d6ServeGated, Fragment: "paths/status/semantic.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/advisories", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/advisory_catalog.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/advisories/evidence", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/advisory_evidence.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/container-images/identities", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/container_images.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/container-images/identities/count", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/container_image_identity_aggregate.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/container-images/identities/inventory", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/container_image_identity_aggregate.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/impact/explain", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/impact_explain.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/impact/findings", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/impact_findings.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/impact/findings/count", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/impact_aggregate.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/impact/inventory", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/impact_aggregate.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/sbom-attestations/attachments", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/sbom_attestations.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/sbom-attestations/attachments/count", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/sbom_attestation_attachment_aggregate.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/sbom-attestations/attachments/inventory", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/sbom_attestation_attachment_aggregate.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/security-alerts/reconciliations", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/security_alerts.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/security-alerts/reconciliations/count", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/security_alert_aggregate.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/security-alerts/reconciliations/inventory", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/security_alert_aggregate.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/vulnerabilities/{advisory_id}", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/advisory_evidence.go"},
	{Method: "GET", Path: "/api/v0/supply-chain/vulnerability-scanner/contract", Family: "supply_chain", Serve: d6ServeGated, Fragment: "paths/supply/chain/vulnerability_scanner_contract.go"},
}

// registerD6Wave mounts the D6 Status + SupplyChain termination wave
// (Refs eshu-hq/eshu#9669): the 32 pinned read routes register
// explicitly inside the D6 wave gate, ahead of the generic proxy. A
// valid shared key terminates on the native lane; a foreign
// credential class (scoped token, OIDC bearer, session cookie)
// relays explicitly on the relay lane so core judges it;
// credential-less callers — and junk-cookie carriers, per the D2
// follow-up lesson — get the standing 401. Each trailing-slash
// spelling stays on the wave route for the same never-split reason.
// The phase-2 extra-port Status legs (index-status, status/index,
// status/operations, behind StatusExtraReadAdmission v0) terminate
// in this lane; the governance, answer-narration, and
// hosted-readiness aspects terminated in the W8a lane (see
// api/w8a_wave.go); the impact packet and the suppressions mutation
// still cross their existing lanes. Compare terminates in its
// gap-04 lane (see api/compare_wave.go). Lives here (not in
// routes()) so the shared router stays under the file-size cap.
func (s *Server) registerD6Wave(mux *http.ServeMux) {
	for _, entry := range d6Wave {
		pattern := entry.Method + " " + entry.Path
		gated := s.instrumented(viaNative, s.d6ReadGate(http.HandlerFunc(s.serveD6Terminated)))
		mux.Handle(pattern, gated)
		mux.Handle(pattern+"/", gated)
	}
}
