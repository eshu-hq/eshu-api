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

// Code-graph relay-leg termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#121). TDD record: the frozen wave fails
// RED before the implementation lands (codeGraphWave undefined), and
// the ahead-of-relay assertions fail next (the 4 graph routes still
// cross the generic relay); both are green with the wave gate
// registered ahead of the generic proxy. Until the lane lands, every
// graph call crosses the generic byte-relay, so the ahead-of-relay
// assertions fail first.

// TestCodeGraphWaveFrozen pins the exact surface the satellite
// terminates: the 4 G-a graph POSTs behind CodeGraphReadAdmission v0
// (the bounded graph traversal plus the entity-name resolution every
// route admits through). A wave change moves the ownership note, the
// fragment reference, and this mirror together — the same frozen-set
// pattern as TestDriftWaveFrozen.
func TestCodeGraphWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/code/call-chain code_graph gated go/internal/query/openapi/paths/code/routes.go",
		"POST /api/v0/code/call-chain/compare code_graph gated go/internal/query/openapi/paths/code/routes.go",
		"POST /api/v0/code/call-graph/metrics code_graph gated go/internal/query/openapi/paths/code/symbols.go",
		"POST /api/v0/code/routes/callers code_graph gated go/internal/query/openapi/paths/code/route_to_caller.go",
	}
	var got []string
	for _, entry := range codeGraphWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("codeGraphWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("codeGraphWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// Wave G-a route paths, shared by the behavior suites.
const (
	codeGraphCallChainPath = "/api/v0/code/call-chain"
	codeGraphComparePath   = "/api/v0/code/call-chain/compare"
	codeGraphMetricsPath   = "/api/v0/code/call-graph/metrics"
	codeGraphRouteCallers  = "/api/v0/code/routes/callers"
	codeGraphFamily        = "code_graph"
)

// codeGraphCallChainBodies are the well-formed request bodies the
// behavior tests drive through the call-chain leg: the repo-scoped
// name pair, the entity-id pair, the cross_repo pair with explicit
// endpoint selectors, and the depth-bounded scoped shape. The
// satellite forwards every shape opaquely; core validates them.
var codeGraphCallChainBodies = []string{
	`{"start":"handleLogin","end":"writeSession","repo_id":"repo-1"}`,
	`{"start_entity_id":"ent-1","end_entity_id":"ent-2","repo_id":"repo-1"}`,
	`{"start":"a","end":"b","cross_repo":true,"start_repo_id":"repo-1","end_repo_id":"repo-2","max_depth":5}`,
	`{"start":"a","end":"b","repo_id":"repo-1","max_depth":3}`,
}

// codeGraphCompareBodies are the well-formed request bodies the
// behavior tests drive through the compare leg: the repo-scoped name
// pair and the entity-id pair with explicit path caps. Single-repo
// v1: repo_id required, cross_repo refused core-side.
var codeGraphCompareBodies = []string{
	`{"start":"handleLogin","end":"writeSession","repo_id":"repo-1"}`,
	`{"start_entity_id":"ent-1","end_entity_id":"ent-2","repo_id":"repo-1","max_depth":4,"max_paths":5}`,
}

// codeGraphMetricsBodies are the well-formed request bodies the
// behavior tests drive through the metrics leg: hub functions and
// recursive functions, each repo-scoped with paging.
var codeGraphMetricsBodies = []string{
	`{"repo_id":"repo-1","metric_type":"hub_functions","limit":25}`,
	`{"repo_id":"repo-1","metric_type":"recursive_functions","limit":10,"offset":5}`,
}

// codeGraphCallersBodies are the well-formed request bodies the
// behavior tests drive through the routes/callers leg: the repo
// scope, the service scope, and the bounded traversal shape.
var codeGraphCallersBodies = []string{
	`{"repo_id":"repo-1","method":"GET","path":"/api/v0/users"}`,
	`{"service_id":"svc-1","method":"POST","path":"/api/v0/orders"}`,
	`{"repo_id":"repo-1","method":"GET","path":"/api/v0/users","max_depth":3,"limit":25}`,
}

// codeGraphAuthedPost builds a shared-key G-a POST with one of the
// parity bodies, the negotiated shape, and a pinned correlation ID.
func codeGraphAuthedPost(path, body, accept string) *http.Request {
	req := authedRequest(http.MethodPost, path)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "graph-parity-1")
	return req
}

// TestCodeGraphWaveAheadOfRelay pins that the wave routes are explicit
// registrations ahead of the generic proxy: a shared-key call must
// file its dispatch on the native lane (viaNative), not the relay
// lane — every leg on the code_graph family. Until the lane lands,
// the calls cross the generic relay and this test is RED.
func TestCodeGraphWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path   string
		bodies []string
	}{
		{codeGraphCallChainPath, codeGraphCallChainBodies},
		{codeGraphComparePath, codeGraphCompareBodies},
		{codeGraphMetricsPath, codeGraphMetricsBodies},
		{codeGraphRouteCallers, codeGraphCallersBodies},
	}
	for _, leg := range legs {
		for _, body := range leg.bodies {
			t.Run("POST "+leg.path, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"chains":[]}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, codeGraphAuthedPost(leg.path, body, "application/json"))

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
				}
				if core.lastBody != body {
					t.Errorf("upstream body = %q, want the graph request %q forwarded intact", core.lastBody, body)
				}
				if core.lastPath != leg.path {
					t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, leg.path)
				}
				snap := tel.snapshot()
				lanes, ok := snap.dispatches[codeGraphFamily]
				if !ok {
					t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", codeGraphFamily)
				}
				if lanes[viaNative] == nil {
					t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", codeGraphFamily, lanes)
				}
				if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
					t.Errorf("family %q native core_ok = %d, want 1", codeGraphFamily, got)
				}
			})
		}
	}
}

// TestCodeGraphSlashSpellingsTerminateNatively pins the never-split
// rule for the wave: every trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestCodeGraphSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	bodies := map[string][]string{
		codeGraphCallChainPath: codeGraphCallChainBodies,
		codeGraphComparePath:   codeGraphCompareBodies,
		codeGraphMetricsPath:   codeGraphMetricsBodies,
		codeGraphRouteCallers:  codeGraphCallersBodies,
	}
	for _, entry := range codeGraphWave {
		t.Run(entry.Method+" "+entry.Path+" slash", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"chains":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := codeGraphAuthedPost(entry.Path, bodies[entry.Path][0], "application/json")
			req.URL.Path = entry.Path + "/"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("%s %s/: status = %d, want 200 from the stub core", entry.Method, entry.Path, rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.dispatches[entry.Family][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", entry.Family, got)
			}
		})
	}
}

// TestCodeGraphTerminated_ForwardsCallerBytes pins that the terminated
// reader forwards the caller's method, path, query, negotiation
// headers, credential, and body to the same core route: exactly one
// upstream hit, one family authorizer allow, no dest retry, no
// credential widening — for the call-chain and routes/callers legs.
func TestCodeGraphTerminated_ForwardsCallerBytes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"chains":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	req := codeGraphAuthedPost(codeGraphCallChainPath, codeGraphCallChainBodies[2], EnvelopeMIME)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (no dest retry)", core.hits)
	}
	if core.lastMethod != http.MethodPost {
		t.Errorf("upstream method = %q, want POST", core.lastMethod)
	}
	if core.lastPath != codeGraphCallChainPath {
		t.Errorf("upstream path = %q, want the wave path", core.lastPath)
	}
	if core.lastBody != codeGraphCallChainBodies[2] {
		t.Errorf("upstream body = %q, want the caller body intact", core.lastBody)
	}
	if core.lastAccept != EnvelopeMIME {
		t.Errorf("upstream Accept = %q, want the negotiated shape intact", core.lastAccept)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization carries the caller credential forward, got %q", core.lastAuth)
	}
	if got := rec.Header().Get("X-Correlation-ID"); got != "graph-parity-1" {
		t.Errorf("response correlation = %q, want the caller value echoed", got)
	}

	callers := codeGraphAuthedPost(codeGraphRouteCallers, codeGraphCallersBodies[0], "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, callers)

	if rec.Code != http.StatusOK {
		t.Fatalf("callers status = %d, want 200 from the stub core", rec.Code)
	}
	if core.hits != 2 {
		t.Errorf("upstream hits = %d, want exactly 2 total (one per call, no dest retry)", core.hits)
	}
	if core.lastPath != codeGraphRouteCallers {
		t.Errorf("upstream path = %q, want the callers wave path", core.lastPath)
	}

	snap := tel.snapshot()
	if got := snap.authorizer[codeGraphFamily][decisionAllow]; got != 2 {
		t.Errorf("authorizer[%s][allow] = %d, want exactly 2 for the terminated calls", codeGraphFamily, got)
	}
}
