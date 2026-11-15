// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// Impact relay-leg termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#97). TDD record: the frozen wave fails
// RED before the implementation lands (impactWave undefined), and the
// ahead-of-relay assertions fail next (the 13 impact POSTs still cross
// the generic relay); both are green with the wave gate registered
// ahead of the generic proxy. Until the lane lands, every impact call
// crosses the generic byte-relay, so the ahead-of-relay assertions
// fail first.

// TestImpactWaveFrozen pins the exact surface the satellite terminates
// plus the two it holds: the 11 proceed legs behind ImpactReadAdmission
// v0 (with the explicit Entity dual-pin cite on the two trace legs)
// plus the two code-surface adapter legs on explicit relay hold. The
// holds keep their standing until core provides satellite-consumable
// adapter seams (TODO-eshu-9669-w5: terminate POST
// /api/v0/impact/change-surface and POST
// /api/v0/impact/change-surface/investigate once the core seam for
// ChangeSurfaceCodeBackend.FetchCodeSurface lands; until then the legs
// relay explicitly and the relay pins in impact_parity_test.go carry
// them). A wave change moves the ownership note, the fragment
// reference, and this mirror together.
func TestImpactWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/impact/blast-radius impact gated go/internal/query/openapi/paths/impact/routes.go",
		"POST /api/v0/impact/change-surface impact relay_hold go/internal/query/openapi/paths/impact/routes.go",
		"POST /api/v0/impact/change-surface/investigate impact relay_hold go/internal/query/openapi/paths/impact/routes.go",
		"POST /api/v0/impact/contracts impact gated go/internal/query/openapi/paths/impact/contract.go",
		"POST /api/v0/impact/deployment-config-influence impact gated go/internal/query/openapi/paths/impact/deployment_config_influence.go",
		"POST /api/v0/impact/developer-change-plan impact gated go/internal/query/openapi/paths/impact/rest.go",
		"POST /api/v0/impact/entity-map impact gated go/internal/query/openapi/paths/impact/rest.go",
		"POST /api/v0/impact/explain-dependency-path impact gated go/internal/query/openapi/paths/impact/rest.go",
		"POST /api/v0/impact/pre-change impact gated go/internal/query/openapi/paths/impact/rest.go",
		"POST /api/v0/impact/resource-investigation impact gated go/internal/query/openapi/paths/impact/rest.go",
		"POST /api/v0/impact/trace-deployment-chain impact gated go/internal/query/openapi/paths/impact/routes.go",
		"POST /api/v0/impact/trace-exposure-path impact gated go/internal/query/openapi/paths/impact/exposure.go",
		"POST /api/v0/impact/trace-resource-to-code impact gated go/internal/query/openapi/paths/impact/rest.go",
	}
	var got []string
	for _, entry := range impactWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("impactWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("impactWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// impactParityLegs drives one representative request per wave row. The
// two trace legs carry a second workload-shaped body: their
// TraceContext leg pulls the F-A service-story closure through the
// root adapter, which the Entity pin covers by dual-pin cite.
var impactParityLegs = []struct {
	key    string
	target string
	body   string
}{
	{"POST /api/v0/impact/trace-deployment-chain", "/api/v0/impact/trace-deployment-chain", `{"service_name":"payments","direct_only":true}`},
	{"POST /api/v0/impact/trace-deployment-chain", "/api/v0/impact/trace-deployment-chain", `{"service_name":"workload:api-svc","direct_only":false,"max_depth":25}`},
	{"POST /api/v0/impact/blast-radius", "/api/v0/impact/blast-radius", `{"service_name":"payments"}`},
	{"POST /api/v0/impact/change-surface", "/api/v0/impact/change-surface", `{"repo_id":"repo_1","relative_paths":["src/a.go"]}`},
	{"POST /api/v0/impact/change-surface/investigate", "/api/v0/impact/change-surface/investigate", `{"repo_id":"repo_1","topic":"auth refresh"}`},
	{"POST /api/v0/impact/contracts", "/api/v0/impact/contracts", `{"service_name":"payments"}`},
	{"POST /api/v0/impact/deployment-config-influence", "/api/v0/impact/deployment-config-influence", `{"service_name":"payments"}`},
	{"POST /api/v0/impact/deployment-config-influence", "/api/v0/impact/deployment-config-influence", `{"service_name":"workload:api-svc"}`},
	{"POST /api/v0/impact/trace-exposure-path", "/api/v0/impact/trace-exposure-path", `{"service_name":"payments"}`},
	{"POST /api/v0/impact/pre-change", "/api/v0/impact/pre-change", `{"service_name":"payments","base_ref":"main"}`},
	{"POST /api/v0/impact/developer-change-plan", "/api/v0/impact/developer-change-plan", `{"service_name":"payments","head_ref":"feature/x"}`},
	{"POST /api/v0/impact/entity-map", "/api/v0/impact/entity-map", `{"repo_id":"repo_1"}`},
	{"POST /api/v0/impact/resource-investigation", "/api/v0/impact/resource-investigation", `{"resource_id":"res_1"}`},
	{"POST /api/v0/impact/trace-resource-to-code", "/api/v0/impact/trace-resource-to-code", `{"resource_id":"res_1"}`},
	{"POST /api/v0/impact/explain-dependency-path", "/api/v0/impact/explain-dependency-path", `{"from_repo":"repo_1","to_repo":"repo_2"}`},
}

// TestImpactWaveAheadOfRelay pins that the wave proceed legs are
// explicit registrations ahead of the generic proxy: a shared-key
// call must file its dispatch on the native lane (viaNative). The two
// relay_hold adapter legs assert the opposite: they file on the relay
// lane explicitly (never the catch-all), per the hold TODO above.
// Until the lane lands, the proceed calls cross the generic relay and
// this test is RED.
func TestImpactWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, leg := range impactParityLegs {
		t.Run(leg.target+" "+leg.body, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := authedRequest(http.MethodPost, leg.target)
			req.Body = io.NopCloser(strings.NewReader(leg.body))
			req.ContentLength = int64(len(leg.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")
			req.Header.Set("X-Correlation-ID", "impact-parity-1")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			entry := impactWaveEntryFor(t, leg.key)
			wantVia := viaNative
			if entry.Serve == impactServeRelayHold {
				wantVia = viaRelay
			}
			snap := tel.snapshot()
			lanes, ok := snap.dispatches[entry.Family]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", entry.Family)
			}
			if wantVia == viaNative && lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", entry.Family, lanes)
			}
			if got := lanes[wantVia][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q %s core_ok = %d, want 1", entry.Family, wantVia, got)
			}
			if core.hits != 1 {
				t.Errorf("upstream hits = %d, want exactly 1 (one server-side fetch, no retry)", core.hits)
			}
		})
	}
}

// impactWaveEntryFor resolves a wave key to its table entry for
// family/serve assertions.
func impactWaveEntryFor(t *testing.T, key string) impactRoute {
	t.Helper()
	for _, entry := range impactWave {
		if entry.Method+" "+entry.Path == key {
			return entry
		}
	}
	t.Fatalf("no impactWave entry for %q", key)
	return impactRoute{}
}
