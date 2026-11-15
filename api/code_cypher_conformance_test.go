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

// Code-cypher relay-leg termination conformance (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#98, W6d). TDD record:
// the frozen wave fails RED before the implementation lands
// (codeCypherWave undefined), and the ahead-of-relay assertions fail
// next (the 2 W6d cypher routes still cross the generic relay); both
// are green with the wave gate registered ahead of the generic proxy.
// Until the lane lands, every cypher call crosses the generic
// byte-relay, so the ahead-of-relay assertions fail first.

// TestCodeCypherWaveFrozen pins the exact surface the satellite
// terminates: the 2 W6d caller-program POSTs behind
// CodeCypherReadAdmission v0 (the bounded graph traversal each route
// admits through, bounded by boundedReadOnlyCypher /
// boundedVisualizationCypher plus the route timeout). A wave change
// moves the ownership note, the fragment reference, and this mirror
// together — the same frozen-set pattern as
// TestRelationshipWaveFrozen.
func TestCodeCypherWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/code/cypher code_cypher gated go/internal/query/openapi/paths/code/graph.go",
		"POST /api/v0/code/visualize code_cypher gated go/internal/query/openapi/paths/code/graph.go",
	}
	var got []string
	for _, entry := range codeCypherWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("codeCypherWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("codeCypherWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// Wave W6d route paths, shared by the behavior suites.
const (
	codeCypherQueryPath     = "/api/v0/code/cypher"
	codeCypherVisualizePath = "/api/v0/code/visualize"
	codeCypherFamily        = "code_cypher"
)

// codeCypherQueryBodies are the well-formed request bodies the
// behavior tests drive through the cypher leg: the plain and the
// limit-pinned queries. The satellite forwards every shape opaquely;
// core validates them.
var codeCypherQueryBodies = []string{
	`{"cypher_query":"MATCH (n:Function) RETURN n LIMIT 10"}`,
	`{"cypher_query":"MATCH (n:Function) RETURN n","limit":25}`,
}

// codeCypherVisualizeBodies are the well-formed request bodies the
// behavior tests drive through the visualize leg: the plain and the
// limit-pinned queries. The satellite forwards every shape opaquely;
// core validates them.
var codeCypherVisualizeBodies = []string{
	`{"cypher_query":"MATCH (n:Function)-[r:CALLS]->(m:Function) RETURN n, r, m"}`,
	`{"cypher_query":"MATCH (n:Function) RETURN n","limit":25}`,
}

// codeCypherAuthedPost builds a shared-key W6d POST with one of the
// parity bodies, the negotiated shape, and a pinned correlation ID.
func codeCypherAuthedPost(path, body, accept string) *http.Request {
	req := authedRequest(http.MethodPost, path)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "code-cypher-parity-1")
	return req
}

// TestCodeCypherWaveAheadOfRelay pins that the wave routes are
// explicit registrations ahead of the generic proxy: a shared-key
// call must file its dispatch on the native lane (viaNative), not
// the relay lane — every leg on the code_cypher family. Until the
// lane lands, the calls cross the generic relay and this test is
// RED.
func TestCodeCypherWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path   string
		bodies []string
	}{
		{codeCypherQueryPath, codeCypherQueryBodies},
		{codeCypherVisualizePath, codeCypherVisualizeBodies},
	}
	for _, leg := range legs {
		for _, body := range leg.bodies {
			t.Run("POST "+leg.path, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, codeCypherAuthedPost(leg.path, body, "application/json"))

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
				}
				if core.lastBody != body {
					t.Errorf("upstream body = %q, want the W6d request %q forwarded intact", core.lastBody, body)
				}
				if core.lastPath != leg.path {
					t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, leg.path)
				}
				snap := tel.snapshot()
				lanes, ok := snap.dispatches[codeCypherFamily]
				if !ok {
					t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", codeCypherFamily)
				}
				if lanes[viaNative] == nil {
					t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", codeCypherFamily, lanes)
				}
				if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
					t.Errorf("family %q native core_ok = %d, want 1", codeCypherFamily, got)
				}
			})
		}
	}
}

// TestCodeCypherSlashSpellingsTerminateNatively pins the never-split
// rule for the wave: every trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestCodeCypherSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	bodies := map[string][]string{
		codeCypherQueryPath:     codeCypherQueryBodies,
		codeCypherVisualizePath: codeCypherVisualizeBodies,
	}
	for _, entry := range codeCypherWave {
		t.Run(entry.Method+" "+entry.Path+" slash", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := codeCypherAuthedPost(entry.Path, bodies[entry.Path][0], "application/json")
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

// TestCodeCypherTerminated_ForwardsCallerBytes pins that the
// terminated reader forwards the caller's method, path, query,
// negotiation headers, credential, and body to the same core route:
// exactly one upstream hit, one family authorizer allow, no dest
// retry, no credential widening — for both W6d legs.
func TestCodeCypherTerminated_ForwardsCallerBytes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	req := codeCypherAuthedPost(codeCypherQueryPath, codeCypherQueryBodies[0], EnvelopeMIME)
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
	if core.lastPath != codeCypherQueryPath {
		t.Errorf("upstream path = %q, want the wave path", core.lastPath)
	}
	if core.lastBody != codeCypherQueryBodies[0] {
		t.Errorf("upstream body = %q, want the caller body intact", core.lastBody)
	}
	if core.lastAccept != EnvelopeMIME {
		t.Errorf("upstream Accept = %q, want the negotiated shape intact", core.lastAccept)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization carries the caller credential forward, got %q", core.lastAuth)
	}
	if got := rec.Header().Get("X-Correlation-ID"); got != "code-cypher-parity-1" {
		t.Errorf("response correlation = %q, want the caller value echoed", got)
	}

	viz := codeCypherAuthedPost(codeCypherVisualizePath, codeCypherVisualizeBodies[0], "application/json")
	vizRec := httptest.NewRecorder()
	handler.ServeHTTP(vizRec, viz)

	if vizRec.Code != http.StatusOK {
		t.Fatalf("visualize status = %d, want 200 from the stub core", vizRec.Code)
	}
	if core.hits != 2 {
		t.Errorf("upstream hits = %d, want 2 (one fetch per leg, no retry)", core.hits)
	}
}
