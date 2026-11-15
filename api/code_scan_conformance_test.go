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

// Code-scan relay-leg termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#98, W6b). TDD record: the frozen wave
// fails RED before the implementation lands (codeScanWave undefined),
// and the ahead-of-relay assertions fail next (the 4 W6b scan routes
// still cross the generic relay); both are green with the wave gate
// registered ahead of the generic proxy. Until the lane lands, every
// scan call crosses the generic byte-relay, so the ahead-of-relay
// assertions fail first.

// TestCodeScanWaveFrozen pins the exact surface the satellite
// terminates: the 4 W6b graph-scan POSTs behind
// CodeGraphScanReadAdmission v0 (the bounded traversal plus the
// by-ID enrichment the complexity by-ID branch hydrates through). A
// wave change moves the ownership note, the fragment reference, and
// this mirror together — the same frozen-set pattern as
// TestRelationshipWaveFrozen.
func TestCodeScanWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/code/bundles code_scan gated go/internal/query/openapi/paths/code/graph.go",
		"POST /api/v0/code/complexity code_scan gated go/internal/query/openapi/paths/code/routes.go",
		"POST /api/v0/code/imports/investigate code_scan gated go/internal/query/openapi/paths/code/symbols.go",
		"POST /api/v0/code/quality/inspect code_scan gated go/internal/query/openapi/paths/code/quality.go",
	}
	var got []string
	for _, entry := range codeScanWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("codeScanWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("codeScanWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// Wave W6b route paths, shared by the behavior suites.
const (
	codeScanComplexityPath = "/api/v0/code/complexity"
	codeScanQualityPath    = "/api/v0/code/quality/inspect"
	codeScanImportsPath    = "/api/v0/code/imports/investigate"
	codeScanBundlesPath    = "/api/v0/code/bundles"
	codeScanFamily         = "code_scan"
)

// codeScanComplexityBodies are the well-formed request bodies the
// behavior tests drive through the complexity leg: the by-name
// lookup, the by-ID lookup, and the ranked list. The satellite
// forwards every shape opaquely; core validates them.
var codeScanComplexityBodies = []string{
	`{"repo_id":"repo-1","function_name":"handleWire"}`,
	`{"repo_id":"repo-1","entity_id":"e1"}`,
	`{"repo_id":"repo-1","limit":5}`,
}

// codeScanQualityBodies are the well-formed request bodies the
// behavior tests drive through the quality/inspect leg: the
// complexity check and the function-length check. The satellite
// forwards every shape opaquely; core validates them.
var codeScanQualityBodies = []string{
	`{"check":"complexity","repo_id":"repo-1","language":"go"}`,
	`{"check":"function_length","repo_id":"repo-1"}`,
}

// codeScanImportsBodies are the well-formed request bodies the
// behavior tests drive through the imports/investigate leg: the
// file-scoped and module-scoped investigation shapes. The satellite
// forwards every shape opaquely; core validates them.
var codeScanImportsBodies = []string{
	`{"query_type":"imports_by_file","source_file":"src/wire/handler.go","repo_id":"repo-1"}`,
	`{"query_type":"module_dependencies","source_module":"wire","repo_id":"repo-1"}`,
}

// codeScanBundlesBodies are the well-formed request bodies the
// behavior tests drive through the bundles leg: the query-scoped
// and ecosystem-scoped shapes. The satellite forwards every shape
// opaquely; core validates them.
var codeScanBundlesBodies = []string{
	`{"query":"wire"}`,
	`{"ecosystem":"go"}`,
}

// codeScanAuthedPost builds a shared-key W6b POST with one of the
// parity bodies, the negotiated shape, and a pinned correlation ID.
func codeScanAuthedPost(path, body, accept string) *http.Request {
	req := authedRequest(http.MethodPost, path)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "code-scan-parity-1")
	return req
}

// TestCodeScanWaveAheadOfRelay pins that the wave routes are
// explicit registrations ahead of the generic proxy: a shared-key
// call must file its dispatch on the native lane (viaNative), not
// the relay lane — every leg on the code_scan family. Until the
// lane lands, the calls cross the generic relay and this test is
// RED.
func TestCodeScanWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path   string
		bodies []string
	}{
		{codeScanComplexityPath, codeScanComplexityBodies},
		{codeScanQualityPath, codeScanQualityBodies},
		{codeScanImportsPath, codeScanImportsBodies},
		{codeScanBundlesPath, codeScanBundlesBodies},
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
				handler.ServeHTTP(rec, codeScanAuthedPost(leg.path, body, "application/json"))

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
				}
				if core.lastBody != body {
					t.Errorf("upstream body = %q, want the W6b request %q forwarded intact", core.lastBody, body)
				}
				if core.lastPath != leg.path {
					t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, leg.path)
				}
				snap := tel.snapshot()
				lanes, ok := snap.dispatches[codeScanFamily]
				if !ok {
					t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", codeScanFamily)
				}
				if lanes[viaNative] == nil {
					t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", codeScanFamily, lanes)
				}
				if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
					t.Errorf("family %q native core_ok = %d, want 1", codeScanFamily, got)
				}
			})
		}
	}
}

// TestCodeScanSlashSpellingsTerminateNatively pins the never-split
// rule for the wave: every trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestCodeScanSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	bodies := map[string][]string{
		codeScanComplexityPath: codeScanComplexityBodies,
		codeScanQualityPath:    codeScanQualityBodies,
		codeScanImportsPath:    codeScanImportsBodies,
		codeScanBundlesPath:    codeScanBundlesBodies,
	}
	for _, entry := range codeScanWave {
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

			req := codeScanAuthedPost(entry.Path, bodies[entry.Path][0], "application/json")
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

// TestCodeScanTerminated_ForwardsCallerBytes pins that the terminated
// reader forwards the caller's method, path, query, negotiation
// headers, credential, and body to the same core route: exactly one
// upstream hit, one family authorizer allow, no dest retry, no
// credential widening — for all four W6b legs.
func TestCodeScanTerminated_ForwardsCallerBytes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	first := codeScanAuthedPost(codeScanComplexityPath, codeScanComplexityBodies[0], EnvelopeMIME)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, first)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (no dest retry)", core.hits)
	}
	if core.lastMethod != http.MethodPost {
		t.Errorf("upstream method = %q, want POST", core.lastMethod)
	}
	if core.lastPath != codeScanComplexityPath {
		t.Errorf("upstream path = %q, want the wave path", core.lastPath)
	}
	if core.lastBody != codeScanComplexityBodies[0] {
		t.Errorf("upstream body = %q, want the caller body intact", core.lastBody)
	}
	if core.lastAccept != EnvelopeMIME {
		t.Errorf("upstream Accept = %q, want the negotiated shape intact", core.lastAccept)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization carries the caller credential forward, got %q", core.lastAuth)
	}
	if got := rec.Header().Get("X-Correlation-ID"); got != "code-scan-parity-1" {
		t.Errorf("response correlation = %q, want the caller value echoed", got)
	}

	for _, leg := range []struct {
		path string
		body string
	}{
		{codeScanQualityPath, codeScanQualityBodies[0]},
		{codeScanImportsPath, codeScanImportsBodies[0]},
		{codeScanBundlesPath, codeScanBundlesBodies[0]},
	} {
		legRec := httptest.NewRecorder()
		handler.ServeHTTP(legRec, codeScanAuthedPost(leg.path, leg.body, "application/json"))
		if legRec.Code != http.StatusOK {
			t.Errorf("POST %s: status = %d, want 200 from the stub core", leg.path, legRec.Code)
		}
	}
	if core.hits != 4 {
		t.Errorf("upstream hits = %d, want 4 (one fetch per leg, no retry)", core.hits)
	}
}
