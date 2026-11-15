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

// Supply-chain impact packet relay-leg termination conformance (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97). TDD record: the
// frozen wave fails RED before the implementation lands (packetWave
// undefined), and the ahead-of-relay assertion fails next (the packet
// GET still crosses the generic relay); both are green with the wave
// gate registered ahead of the generic proxy. Until the lane lands,
// the packet call crosses the generic byte-relay, so the
// ahead-of-relay assertion fails first.

// TestPacketWaveFrozen pins the exact surface the satellite
// terminates: the W5f packet single behind
// SupplyChainImpactPacketReadAdmission v0. Decision 2 proof closes in
// this lane (the 7-case suppression corpus in packet_parity_test.go),
// it is not held. A wave change moves the ownership note, the fragment
// reference, and this mirror together.
func TestPacketWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/investigations/supply-chain/impact/packet supply_chain gated go/internal/query/openapi/paths/evidence/investigations.go",
	}
	var got []string
	for _, entry := range packetWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("packetWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("packetWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// packetConcreteTarget is the concrete packet target the behavior
// tests drive.
const packetConcreteTarget = "/api/v0/investigations/supply-chain/impact/packet?scope_id=scope_1"

// TestPacketWaveAheadOfRelay pins that the packet leg is an explicit
// registration ahead of the generic proxy: a shared-key call must
// file its dispatch on the native lane (viaNative), not the relay
// lane. Until the lane lands, the call crosses the generic relay and
// this test is RED.
func TestPacketWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"packet":{},"truth":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, packetConcreteTarget)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Correlation-ID", "packet-parity-1")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
	}
	got, _ := io.ReadAll(rec.Result().Body)
	if string(got) != core.body {
		t.Fatalf("body = %q, want the untouched core bytes %q", got, core.body)
	}
	snap := tel.snapshot()
	lanes, ok := snap.dispatches["supply_chain"]
	if !ok {
		t.Fatalf("family supply_chain has no dispatch rows, want the explicit wave route ahead of relay")
	}
	if lanes[viaNative] == nil {
		t.Errorf("family supply_chain dispatches = %v, want a native lane row for the shared-key wave call", lanes)
	}
	if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
		t.Errorf("family supply_chain native core_ok = %d, want 1", got)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (one server-side fetch, no retry)", core.hits)
	}
	if core.lastPath != "/api/v0/investigations/supply-chain/impact/packet" {
		t.Errorf("upstream path = %q, want the wave route", core.lastPath)
	}
	if core.lastQuery != "scope_id=scope_1" {
		t.Errorf("upstream query = %q, want the caller query preserved", core.lastQuery)
	}
}
