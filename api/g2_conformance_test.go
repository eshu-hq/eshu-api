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

// G2 reads termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#101 RE:G2V2). TDD record: this suite
// failed RED before the implementation landed (frozen wave asserted
// first; the swap-mutant legs relayed, so the ahead-of-relay
// assertions failed; the unknown route had no handler, so the
// fail-closed assertion failed) and is green with it — the frozen
// wave below mirrors the 7 CUTOVER G2 rows behind the 5 core
// read-admission v0 pins (core eshu-hq/eshu#10796), and the behavior
// tests require every wave route to terminate natively for
// shared-key callers while unresolvable identity classes relay
// explicitly.

// TestG2WaveFrozen pins the exact G2 surface the satellite must
// terminate. A wave change (route added, removed, renamed, or
// rehomed) fails here until the ownership map, the spec fragments,
// and this mirror move together — the same frozen-set pattern as
// TestCoreReadContractsVersioned. The want table is the verbatim
// ownership-map rows (method, path, satellite serve family) plus the
// satellite serve kind and the Decision 5(c) core fragment that
// carries the route's spec entry. Every leg classifies to the shared
// "other" family in classifyFamily (no per-family prefix entries
// exist for these paths): the authorizer and telemetry pin that
// value until family entries land, exactly like the Wave-1b
// component-extension legs. The CUTOVER families ride the wave
// comments and contracts/g2-wave.md, not the table.
func TestG2WaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/collector-extraction-readiness other gated paths/status/collector_extraction_readiness.go",
		"GET /api/v0/collector-extraction-readiness/{family} other gated paths/status/collector_extraction_readiness.go",
		"GET /api/v0/fact-schema-versions other gated paths/status/fact_schema_version.go",
		"GET /api/v0/fact-schema-versions/{fact_kind} other gated paths/status/fact_schema_version.go",
		"GET /api/v0/surface-inventory other gated paths/catalog/surface_inventory.go",
		"POST /api/v0/visualizations/derive other gated paths/evidence/visualization_packets.go",
		"GET /api/v0/repository/manifest-dependencies other gated paths/repository/manifest_dependencies.go",
	}
	var got []string
	for _, entry := range g2Wave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("g2Wave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("g2Wave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// g2ConcretePaths fills the wave's path set so the behavior tests
// drive real mux matches. Trailing-slash spellings ride the same
// native registration (never-split), so each entry also names its
// slash variant where the mux distinguishes it. The two wildcard
// legs map to one concrete instance each; the mux pattern keeps the
// {family} and {fact_kind} spellings the pin admits.
var g2ConcretePaths = map[string]string{
	"GET /api/v0/collector-extraction-readiness":          "/api/v0/collector-extraction-readiness",
	"GET /api/v0/collector-extraction-readiness/{family}": "/api/v0/collector-extraction-readiness/repo",
	"GET /api/v0/fact-schema-versions":                    "/api/v0/fact-schema-versions",
	"GET /api/v0/fact-schema-versions/{fact_kind}":        "/api/v0/fact-schema-versions/file",
	"GET /api/v0/surface-inventory":                       "/api/v0/surface-inventory",
	"POST /api/v0/visualizations/derive":                  "/api/v0/visualizations/derive",
	"GET /api/v0/repository/manifest-dependencies":        "/api/v0/repository/manifest-dependencies",
}

// TestG2WaveRoutesAheadOfRelay pins that every wave route is an
// explicit registration ahead of the generic proxy: a shared-key
// call must file its dispatch on the native lane (viaNative), not
// the relay lane. This test is the swap-mutant detector for
// dispatch: if the registration (or the terminated handler behind
// it) is swapped back to the generic dispatch, every leg files
// relay and this test goes RED. Until the lane lands, every call
// crosses the generic byte-relay and this test is RED.
func TestG2WaveRoutesAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, entry := range g2Wave {
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
			target, ok := g2ConcretePaths[key]
			if !ok {
				t.Fatalf("no concrete path for wave route %q", key)
			}
			var req *http.Request
			if entry.Method == http.MethodPost {
				req = g2AuthedReq(entry.Method, target, `{"packet":"derive-probe"}`)
			} else {
				req = g2AuthedReq(entry.Method, target, "")
			}
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

// TestG2UnknownRouteFailClosed pins the terminated-handler switch
// default: any route the wave table does not know answers 404 and
// never rides a leg. Routes() only passes g2Wave entries, so this
// guards the future table-add-without-a-handler mistake.
func TestG2UnknownRouteFailClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, route := range []string{
		"/api/v0/collector-extraction-readiness/bogus/extra",
		"/api/v0/fact-schema-versions/bogus-kind",
		"/api/v0/visualizations/derive/extra",
		"/api/v0/unregistered-route",
		"",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v0/surface-inventory", nil)
		srv.g2TerminatedHandler(route)(rec, req)

		res := rec.Result()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("route %q: status = %d, want the fail-closed 404", route, res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		if len(body) == 0 {
			t.Errorf("route %q: empty body, want the documented unknown-route error", body)
		}
	}
}

// TestG2SlashSpellingsTerminateNatively pins the never-split rule
// for the wave: every trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestG2SlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for key, concrete := range g2ConcretePaths {
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

			entry := g2WaveEntry(key)
			var req *http.Request
			if entry.Method == http.MethodPost {
				req = g2AuthedReq(entry.Method, concrete+"/", `{"packet":"derive-probe"}`)
			} else {
				req = g2AuthedReq(entry.Method, concrete+"/", "")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("%s %s/: status = %d, want 200 from the stub core", entry.Method, concrete, rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(concrete)
			if got := snap.dispatches[family][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", family, got)
			}
		})
	}
}

// g2WaveEntry returns the wave entry for a "METHOD path" key. Tests
// only pass keys from g2ConcretePaths, so a missing key is a test
// bug, not a wave gap.
func g2WaveEntry(key string) g2Route {
	for _, entry := range g2Wave {
		if entry.Method+" "+entry.Path == key {
			return entry
		}
	}
	panic("g2WaveEntry: no wave entry for " + key)
}

// g2AuthedReq builds a shared-key request with an optional JSON
// body. The derive leg is a POST: the body rides the terminated
// lane opaquely to core, never parsed dest-side.
func g2AuthedReq(method, path, body string) *http.Request {
	req := g2UnauthedReq(method, path, body)
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	return req
}

// g2UnauthedReq builds a keyless request with an optional JSON body
// for the gate-shape tests (relay-explicit, junk-cookie,
// credential-less): no Authorization header rides along, so the gate
// judges the presented credential shape alone.
func g2UnauthedReq(method, path, body string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	return req
}
