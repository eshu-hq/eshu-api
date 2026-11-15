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

// Entity relay-leg termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#97). TDD record: the frozen wave fails
// RED before the implementation lands (entityWave undefined), and the
// ahead-of-relay assertions fail next (the 8 entity routes still cross
// the generic relay); both are green with the wave gate registered
// ahead of the generic proxy. Until the lane lands, every entity call
// crosses the generic byte-relay, so the ahead-of-relay assertions
// fail first.

// TestEntityWaveFrozen pins the exact surface the satellite
// terminates plus the two it holds: the 7 proceed legs behind
// EntityReadAdmission v0 plus the context fallback leg and the
// investigations service leg on explicit relay hold. The holds keep
// their standing until core provides a satellite-consumable
// content-fallback satisfier
// (TODO-eshu-9669-w5: terminate GET
// /api/v0/entities/{entity_id}/context once the core satisfier for
// ContentRelationshipBuilder.BuildContentRelationships lands; until
// then the leg relays explicitly and the relay pin in
// entity_parity_test.go carries it) and extends EntityReadAdmission
// with the three investigations reads
// (TODO-eshu-9669-w5: terminate GET
// /api/v0/investigations/services/{service_name} natively once the
// pin covers ListRepoEntitiesByTypes, RepositoryReadModelSummary,
// and SearchFileReferenceAnyRepo). A wave change moves the
// ownership note, the fragment reference, and this mirror together.
func TestEntityWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/entities/{entity_id}/context entities relay_hold go/internal/query/openapi/paths/search/entities.go",
		"GET /api/v0/investigations/services/{service_name} service_intel relay_hold go/internal/query/openapi/paths/search/entities.go",
		"GET /api/v0/services/{service_name}/api-surface/endpoints service_intel gated go/internal/query/openapi/paths/search/entities.go",
		"GET /api/v0/services/{service_name}/context service_intel gated go/internal/query/openapi/paths/search/entities.go",
		"GET /api/v0/services/{service_name}/downstream-consumers service_intel gated go/internal/query/openapi/paths/search/entities.go",
		"GET /api/v0/services/{service_name}/story service_intel gated go/internal/query/openapi/paths/search/entities.go",
		"GET /api/v0/workloads/{workload_id}/context workloads gated go/internal/query/openapi/paths/search/entities.go",
		"GET /api/v0/workloads/{workload_id}/story workloads gated go/internal/query/openapi/paths/search/entities.go",
		"POST /api/v0/entities/resolve entities gated go/internal/query/openapi/paths/search/entities.go",
	}
	var got []string
	for _, entry := range entityWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("entityWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("entityWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// entityConcretePaths maps each wave pattern to the concrete path the
// behavior tests drive (wildcards resolved to fixed test values).
var entityConcretePaths = map[string]string{
	"POST /api/v0/entities/resolve":                             "/api/v0/entities/resolve",
	"GET /api/v0/entities/{entity_id}/context":                  "/api/v0/entities/ent_1/context",
	"GET /api/v0/workloads/{workload_id}/context":               "/api/v0/workloads/wl_1/context",
	"GET /api/v0/workloads/{workload_id}/story":                 "/api/v0/workloads/wl_1/story",
	"GET /api/v0/services/{service_name}/context":               "/api/v0/services/payments/context",
	"GET /api/v0/services/{service_name}/story":                 "/api/v0/services/payments/story",
	"GET /api/v0/services/{service_name}/api-surface/endpoints": "/api/v0/services/payments/api-surface/endpoints",
	"GET /api/v0/services/{service_name}/downstream-consumers":  "/api/v0/services/payments/downstream-consumers",
	"GET /api/v0/investigations/services/{service_name}":        "/api/v0/investigations/services/payments",
}

// TestEntityWaveAheadOfRelay pins that the wave proceed legs are
// explicit registrations ahead of the generic proxy: a shared-key
// call must file its dispatch on the native lane (viaNative), not
// the relay lane. The relay_hold context leg asserts the opposite:
// it files on the relay lane explicitly (never the catch-all), per
// the hold TODO above. Until the lane lands, the proceed calls cross
// the generic relay and this test is RED.
func TestEntityWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, entry := range entityWave {
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
			target := entityConcretePaths[key]
			var req *http.Request
			if entry.Method == http.MethodPost {
				req = authedRequest(http.MethodPost, target)
				body := `{"name":"handleWire","entity_type":"Function"}`
				req.Body = io.NopCloser(strings.NewReader(body))
				req.ContentLength = int64(len(body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = authedRequest(http.MethodGet, target)
			}
			req.Header.Set("Accept", "application/json")
			req.Header.Set("X-Correlation-ID", "entity-parity-1")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			wantVia := viaNative
			if entry.Serve == entityServeRelayHold {
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
