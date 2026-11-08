// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

// D6 Status + SupplyChain reads conformance (Refs eshu-hq/eshu#9669,
// part of #6707). TDD record: this suite failed RED before the
// implementation landed (frozen wave asserted first) and is green with
// it — the frozen wave below mirrors go/cmd/api/CUTOVER_OWNERSHIP.md
// (Status and SupplyChain read rows; Compare deferred with a
// deliberate later pin; the suppressions mutation row stays relay via
// SuppressionAdmission) and the behavior tests require every wave
// route to terminate natively for shared-key callers while
// unresolvable identity classes relay explicitly. Until the lane
// lands, the wave registers nothing and every wave call crosses the
// generic byte-relay (or the key gate), so the ahead-of-relay
// assertions fail first. That red is the recorded expectation, not a
// defect.

// TestD6WaveFrozen pins the exact Status + SupplyChain surface the
// satellite must terminate. A wave change (route added, removed,
// renamed, or rehomed) fails here until the ownership map, the spec
// fragments, and this mirror move together — the same frozen-set
// pattern as TestCoreReadContractsVersioned. The want table is the
// verbatim ownership-map rows (method, path, family) plus the
// satellite serve kind and the Decision 5(c) fragment that carries the
// route's spec entry. Still-relayed rows (the remaining extra-port
// Status aspects, the impact packet, Compare, the suppressions
// mutation) are NOT in this table;
// contracts/d6-status-supplychain.md discloses each with its reason.
func TestD6WaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/collector-readiness status gated paths/status/collector_readiness.go",
		"GET /api/v0/collectors status gated paths/status/status_reads.go",
		"GET /api/v0/index-status status gated paths/status/status_extra_reads.go",
		"GET /api/v0/ingesters status gated paths/status/status_reads.go",
		"GET /api/v0/ingesters/{ingester} status gated paths/status/status_reads.go",
		"GET /api/v0/status/collector-readiness status gated paths/status/collector_readiness.go",
		"GET /api/v0/status/collectors status gated paths/status/status_reads.go",
		"GET /api/v0/status/freshness-causality status gated paths/freshness/causality.go",
		"GET /api/v0/status/index status gated paths/status/status_extra_reads.go",
		"GET /api/v0/status/ingesters status gated paths/status/status_reads.go",
		"GET /api/v0/status/ingesters/{ingester} status gated paths/status/status_reads.go",
		"GET /api/v0/status/operations status gated paths/status/operations.go",
		"GET /api/v0/status/operator-control-plane status gated paths/status/operator_control_plane.go",
		"GET /api/v0/status/pipeline status gated paths/status/status_reads.go",
		"GET /api/v0/status/semantic-extraction status gated paths/status/semantic.go",
		"GET /api/v0/supply-chain/advisories supply_chain gated paths/supply/chain/advisory_catalog.go",
		"GET /api/v0/supply-chain/advisories/evidence supply_chain gated paths/supply/chain/advisory_evidence.go",
		"GET /api/v0/supply-chain/container-images/identities supply_chain gated paths/supply/chain/container_images.go",
		"GET /api/v0/supply-chain/container-images/identities/count supply_chain gated paths/supply/chain/container_image_identity_aggregate.go",
		"GET /api/v0/supply-chain/container-images/identities/inventory supply_chain gated paths/supply/chain/container_image_identity_aggregate.go",
		"GET /api/v0/supply-chain/impact/explain supply_chain gated paths/supply/chain/impact_explain.go",
		"GET /api/v0/supply-chain/impact/findings supply_chain gated paths/supply/chain/impact_findings.go",
		"GET /api/v0/supply-chain/impact/findings/count supply_chain gated paths/supply/chain/impact_aggregate.go",
		"GET /api/v0/supply-chain/impact/inventory supply_chain gated paths/supply/chain/impact_aggregate.go",
		"GET /api/v0/supply-chain/sbom-attestations/attachments supply_chain gated paths/supply/chain/sbom_attestations.go",
		"GET /api/v0/supply-chain/sbom-attestations/attachments/count supply_chain gated paths/supply/chain/sbom_attestation_attachment_aggregate.go",
		"GET /api/v0/supply-chain/sbom-attestations/attachments/inventory supply_chain gated paths/supply/chain/sbom_attestation_attachment_aggregate.go",
		"GET /api/v0/supply-chain/security-alerts/reconciliations supply_chain gated paths/supply/chain/security_alerts.go",
		"GET /api/v0/supply-chain/security-alerts/reconciliations/count supply_chain gated paths/supply/chain/security_alert_aggregate.go",
		"GET /api/v0/supply-chain/security-alerts/reconciliations/inventory supply_chain gated paths/supply/chain/security_alert_aggregate.go",
		"GET /api/v0/supply-chain/vulnerabilities/{advisory_id} supply_chain gated paths/supply/chain/advisory_evidence.go",
		"GET /api/v0/supply-chain/vulnerability-scanner/contract supply_chain gated paths/supply/chain/vulnerability_scanner_contract.go",
	}
	var got []string
	for _, entry := range d6Wave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("d6Wave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("d6Wave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// d6ConcretePaths fills the wave's path wildcards so the behavior
// tests drive real mux matches. Trailing-slash spellings ride the same
// native registration (never-split), so each entry also names its slash
// variant where the mux distinguishes it.
var d6ConcretePaths = map[string]string{
	"GET /api/v0/collector-readiness":                                    "/api/v0/collector-readiness",
	"GET /api/v0/collectors":                                             "/api/v0/collectors",
	"GET /api/v0/ingesters":                                              "/api/v0/ingesters",
	"GET /api/v0/ingesters/{ingester}":                                   "/api/v0/ingesters/repository",
	"GET /api/v0/index-status":                                           "/api/v0/index-status",
	"GET /api/v0/status/collector-readiness":                             "/api/v0/status/collector-readiness",
	"GET /api/v0/status/collectors":                                      "/api/v0/status/collectors",
	"GET /api/v0/status/freshness-causality":                             "/api/v0/status/freshness-causality",
	"GET /api/v0/status/ingesters":                                       "/api/v0/status/ingesters",
	"GET /api/v0/status/ingesters/{ingester}":                            "/api/v0/status/ingesters/repository",
	"GET /api/v0/status/index":                                           "/api/v0/status/index",
	"GET /api/v0/status/operations":                                      "/api/v0/status/operations",
	"GET /api/v0/status/operator-control-plane":                          "/api/v0/status/operator-control-plane",
	"GET /api/v0/status/pipeline":                                        "/api/v0/status/pipeline",
	"GET /api/v0/status/semantic-extraction":                             "/api/v0/status/semantic-extraction",
	"GET /api/v0/supply-chain/advisories":                                "/api/v0/supply-chain/advisories",
	"GET /api/v0/supply-chain/advisories/evidence":                       "/api/v0/supply-chain/advisories/evidence",
	"GET /api/v0/supply-chain/container-images/identities":               "/api/v0/supply-chain/container-images/identities",
	"GET /api/v0/supply-chain/container-images/identities/count":         "/api/v0/supply-chain/container-images/identities/count",
	"GET /api/v0/supply-chain/container-images/identities/inventory":     "/api/v0/supply-chain/container-images/identities/inventory",
	"GET /api/v0/supply-chain/impact/explain":                            "/api/v0/supply-chain/impact/explain",
	"GET /api/v0/supply-chain/impact/findings":                           "/api/v0/supply-chain/impact/findings",
	"GET /api/v0/supply-chain/impact/findings/count":                     "/api/v0/supply-chain/impact/findings/count",
	"GET /api/v0/supply-chain/impact/inventory":                          "/api/v0/supply-chain/impact/inventory",
	"GET /api/v0/supply-chain/sbom-attestations/attachments":             "/api/v0/supply-chain/sbom-attestations/attachments",
	"GET /api/v0/supply-chain/sbom-attestations/attachments/count":       "/api/v0/supply-chain/sbom-attestations/attachments/count",
	"GET /api/v0/supply-chain/sbom-attestations/attachments/inventory":   "/api/v0/supply-chain/sbom-attestations/attachments/inventory",
	"GET /api/v0/supply-chain/security-alerts/reconciliations":           "/api/v0/supply-chain/security-alerts/reconciliations",
	"GET /api/v0/supply-chain/security-alerts/reconciliations/count":     "/api/v0/supply-chain/security-alerts/reconciliations/count",
	"GET /api/v0/supply-chain/security-alerts/reconciliations/inventory": "/api/v0/supply-chain/security-alerts/reconciliations/inventory",
	"GET /api/v0/supply-chain/vulnerabilities/{advisory_id}":             "/api/v0/supply-chain/vulnerabilities/GHSA-1234-5678-abcd",
	"GET /api/v0/supply-chain/vulnerability-scanner/contract":            "/api/v0/supply-chain/vulnerability-scanner/contract",
}

// TestD6WaveRoutesAheadOfRelay pins that every wave route is an
// explicit registration ahead of the generic proxy: a shared-key call
// must file its dispatch on the native lane (viaNative), not the relay
// lane. Until the lane lands, every call crosses the generic relay and
// this test is RED.
func TestD6WaveRoutesAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, entry := range d6Wave {
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			key := entry.Method + " " + entry.Path
			target, ok := d6ConcretePaths[key]
			if !ok {
				t.Fatalf("no concrete path for wave route %q", key)
			}
			req := authedRequest(entry.Method, target)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(target)
			if family != entry.Family {
				t.Fatalf("classifyFamily(%q) = %q, want wave family %q", target, family, entry.Family)
			}
			lanes, ok := snap.dispatches[family]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", family)
			}
			if lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", family, lanes)
			}
			if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1", family, got)
			}
		})
	}
}

// TestD6SuppressionsMutationNotInWave pins the Decision 2 split: the
// suppressions mutation row is the Never-satellite survivor via
// SuppressionAdmission (already on its tier-C write-through lane) and
// must never enter this read wave. Its behavior stays pinned by the
// suppression suite; this test only guards the wave boundary.
func TestD6SuppressionsMutationNotInWave(t *testing.T) {
	t.Parallel()

	for _, entry := range d6Wave {
		if entry.Path == "/api/v0/supply-chain/impact/suppressions" {
			t.Errorf("suppressions mutation is in d6Wave, want it out (SuppressionAdmission survivor, not a read port)")
		}
	}
}

// TestD6RelayedRoutesStayRelay pins the scoping line: the Status
// aspects the pins do not cover, the impact packet, and Compare are
// NOT wave routes and must not file a native dispatch. A shared-key
// call to each still answers from the stub core on the generic relay
// lane — the pin never widens silently. (The suppressions mutation
// above stays on its tier-C write-through lane, covered separately.)
func TestD6RelayedRoutesStayRelay(t *testing.T) {
	t.Parallel()

	relayed := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v0/status/governance", ""},
		{http.MethodGet, "/api/v0/status/answer-narration", ""},
		{http.MethodGet, "/api/v0/status/hosted-readiness", ""},
		{http.MethodGet, "/api/v0/investigations/supply-chain/impact/packet", ""},
		{http.MethodPost, "/api/v0/compare/environments", `{"baseline":"a","candidate":"b"}`},
	}
	for _, route := range relayed {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			t.Parallel()

			for _, entry := range d6Wave {
				if entry.Method+" "+entry.Path == route.method+" "+route.path {
					t.Fatalf("relayed route %q is in d6Wave, want it out (pin does not cover it)", route.method+" "+route.path)
				}
			}
			key := route.method + " " + route.path
			if _, ok := d6ConcretePaths[key]; ok {
				t.Fatalf("relayed route %q has a concrete wave path, want it out of the wave harness", key)
			}

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := authedRequest(route.method, route.path)
			if route.body != "" {
				req.Body = io.NopCloser(http.NoBody)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(route.path)
			lanes := snap.dispatches[family]
			if lanes[viaNative] != nil {
				t.Errorf("family %q dispatches = %v, want no native lane row for the relayed route", family, lanes)
			}
		})
	}
}
