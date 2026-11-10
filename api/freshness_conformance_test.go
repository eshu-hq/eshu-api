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

// Wave-1 Freshness reads conformance (Refs eshu-hq/eshu#9669, D13
// tracker eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#72). TDD
// record: this suite failed RED before the implementation landed
// (frozen wave asserted first; the swap-mutant legs relayed, so the
// ahead-of-relay assertions failed; the unknown route rode a leg, so
// the fail-closed assertion failed) and is green with it — the frozen
// wave below mirrors the CUTOVER_OWNERSHIP Freshness rows behind
// FreshnessReadAdmission v0, and the behavior tests require every wave
// route to terminate natively for shared-key callers while
// unresolvable identity classes relay explicitly.

// TestFreshnessWaveFrozen pins the exact Freshness surface the
// satellite must terminate. A wave change (route added, removed,
// renamed, or rehomed) fails here until the ownership map, the spec
// fragments, and this mirror move together — the same frozen-set
// pattern as TestCoreReadContractsVersioned. The want table is the
// verbatim ownership-map rows (method, path, family) plus the
// satellite serve kind and the Decision 5(c) fragment that carries the
// route's spec entry. Still-relayed freshness-path rows (the D4
// repository freshness read, the D4 documentation freshness reads, the
// D6 freshness-causality read) are NOT in this table — they terminated
// in their own lanes with their own families;
// contracts/freshness-wave.md discloses each with its reason.
func TestFreshnessWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/freshness/changed-since freshness gated paths/freshness/changed_since.go",
		"GET /api/v0/freshness/generations freshness gated paths/freshness/generations.go",
		"GET /api/v0/freshness/services/changed-since freshness gated paths/freshness/service_changed_since.go",
	}
	var got []string
	for _, entry := range freshnessWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("freshnessWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("freshnessWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// freshnessConcretePaths fills the wave's path set so the behavior
// tests drive real mux matches. Trailing-slash spellings ride the same
// native registration (never-split), so each entry also names its slash
// variant where the mux distinguishes it. The wave carries no path
// wildcards, so every entry maps to itself.
var freshnessConcretePaths = map[string]string{
	"GET /api/v0/freshness/changed-since":          "/api/v0/freshness/changed-since",
	"GET /api/v0/freshness/generations":            "/api/v0/freshness/generations",
	"GET /api/v0/freshness/services/changed-since": "/api/v0/freshness/services/changed-since",
}

// TestFreshnessWaveRoutesAheadOfRelay pins that every wave route is an
// explicit registration ahead of the generic proxy: a shared-key call
// must file its dispatch on the native lane (viaNative), not the relay
// lane. This test is the swap-mutant detector for dispatch: if the
// registration (or the terminated handler behind it) is swapped back
// to the generic dispatch, every leg files relay and this test goes
// RED. Until the lane lands, every call crosses the generic byte-relay
// and this test is RED.
func TestFreshnessWaveRoutesAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, entry := range freshnessWave {
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
			target, ok := freshnessConcretePaths[key]
			if !ok {
				t.Fatalf("no concrete path for wave route %q", key)
			}
			// The changed-since legs carry filter queries core-side;
			// the gate and the dispatch forward them untouched, so
			// the ahead-of-relay proof drives one representative
			// query per leg.
			switch target {
			case freshnessChangedSincePath:
				target += "?scope_id=scope_1&since_generation_id=gen_9"
			case freshnessGenerationsPath:
				target += "?scope_id=scope_1&limit=10"
			case freshnessServiceChangedSincePath:
				target += "?service_id=svc_1&since_generation_id=gen_9"
			}
			req := authedRequest(entry.Method, target)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(entry.Path)
			if family != entry.Family {
				t.Fatalf("classifyFamily(%q) = %q, want wave family %q", entry.Path, family, entry.Family)
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

// TestFreshnessUnknownRouteFailClosed pins the terminated-handler
// switch default: any route the wave table does not know answers 404
// and never rides a leg. Routes() only passes freshnessWave entries,
// so this guards the future table-add-without-a-handler mistake.
func TestFreshnessUnknownRouteFailClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, route := range []string{
		"GET /api/v0/freshness/bogus",
		"GET /api/v0/freshness/changed-since/extra",
		"POST /api/v0/freshness/generations",
		"",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v0/freshness/generations", nil)
		srv.freshnessTerminatedHandler(route)(rec, req)

		res := rec.Result()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("route %q: status = %d, want the fail-closed 404", route, res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		if len(body) == 0 {
			t.Errorf("route %q: empty body, want the documented unknown-route error", route)
		}
	}
}

// TestFreshnessOtherFreshnessPathsNotInWave pins the wave boundary:
// the freshness-path reads that terminated in earlier lanes keep their
// own families and never enter this wave. The /freshness family prefix
// must not hijack them: each still classifies to its owning family.
func TestFreshnessOtherFreshnessPathsNotInWave(t *testing.T) {
	t.Parallel()

	others := []struct {
		method string
		path   string
		family string
	}{
		{http.MethodGet, "/api/v0/repositories/repo_1/freshness", "repositories"},
		{http.MethodGet, "/api/v0/documentation/evidence-packets/pkt_1/freshness", "documentation"},
		{http.MethodGet, "/api/v0/status/freshness-causality", "status"},
	}
	for _, tc := range others {
		if got := classifyFamily(tc.path); got != tc.family {
			t.Errorf("classifyFamily(%q) = %q, want owning family %q (never hijacked into freshness)", tc.path, got, tc.family)
		}
		for _, entry := range freshnessWave {
			if entry.Method+" "+entry.Path == tc.method+" "+tc.path {
				t.Errorf("earlier-lane route %q is in freshnessWave, want it out", tc.method+" "+tc.path)
			}
		}
	}
}

// TestFreshnessSlashSpellingsTerminateNatively pins the never-split
// rule for the wave: every trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestFreshnessSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for key, concrete := range freshnessConcretePaths {
		t.Run(key+" slash", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, authedRequest(http.MethodGet, concrete+"/"))

			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s/: status = %d, want 200 from the stub core", concrete, rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(concrete)
			if got := snap.dispatches[family][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", family, got)
			}
		})
	}
}
