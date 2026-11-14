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

// Code-divergence relay-leg termination conformance (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#125). TDD record: the
// frozen wave fails RED before the implementation lands
// (codeDivergenceWave undefined), and the ahead-of-relay assertions
// fail next (the 3 divergence routes still cross the generic relay);
// both are green with the wave gate registered ahead of the generic
// proxy. Until the lane lands, every divergence call crosses the
// generic byte-relay, so the ahead-of-relay assertions fail first.

// TestCodeDivergenceWaveFrozen pins the exact surface the satellite
// terminates: the 3 divergence POSTs behind CodeDivergenceReadAdmission
// v0 (the bounded content reads plus the wrapper/outlier graph track
// every route admits through). A wave change moves the ownership
// note, the fragment reference, and this mirror together — the same
// frozen-set pattern as TestCodeGraphWaveFrozen.
func TestCodeDivergenceWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/code/divergence/findings code_divergence gated go/internal/query/openapi/paths/code/divergence.go",
		"POST /api/v0/code/divergence/investigate code_divergence gated go/internal/query/openapi/paths/code/divergence.go",
		"POST /api/v0/code/divergence/report code_divergence gated go/internal/query/openapi/paths/code/divergence.go",
	}
	var got []string
	for _, entry := range codeDivergenceWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("codeDivergenceWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("codeDivergenceWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// Wave divergence route paths, shared by the behavior suites.
const (
	codeDivergenceFindingsPath    = "/api/v0/code/divergence/findings"
	codeDivergenceInvestigatePath = "/api/v0/code/divergence/investigate"
	codeDivergenceReportPath      = "/api/v0/code/divergence/report"
	codeDivergenceFamily          = "code_divergence"
)

// codeDivergenceFindingsBodies are the well-formed request bodies the
// behavior tests drive through the findings leg: the kind-selected
// page and the paged default page. The satellite forwards every shape
// opaquely; core validates them.
var codeDivergenceFindingsBodies = []string{
	`{"repo_id":"repo-1","kind":"exact"}`,
	`{"repo_id":"repo-1","limit":25,"offset":25}`,
	`{"repo_id":"repo-1","kind":"drifted"}`,
}

// codeDivergenceInvestigateBodies are the well-formed request bodies
// the behavior tests drive through the investigate leg: the exact,
// drifted, and qualified-kind point lookups.
var codeDivergenceInvestigateBodies = []string{
	`{"repo_id":"repo-1","kind":"exact","fingerprint":"fp-1"}`,
	`{"repo_id":"repo-1","kind":"drifted","fingerprint":"drift-1"}`,
	`{"repo_id":"repo-1","kind":"parallel_implementation.exact","fingerprint":"fp-1"}`,
}

// codeDivergenceReportBodies are the well-formed request bodies the
// behavior tests drive through the report leg: the default rollup and
// the top-bounded rollup.
var codeDivergenceReportBodies = []string{
	`{"repo_id":"repo-1"}`,
	`{"repo_id":"repo-1","top_per_kind":1}`,
}

// codeDivergenceAuthedPost builds a shared-key divergence POST with
// one of the parity bodies, the negotiated shape, and a pinned
// correlation ID.
func codeDivergenceAuthedPost(path, body, accept string) *http.Request {
	req := authedRequest(http.MethodPost, path)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "divergence-parity-1")
	return req
}

// TestCodeDivergenceWaveAheadOfRelay pins that the wave routes are
// explicit registrations ahead of the generic proxy: a shared-key
// call must file its dispatch on the native lane (viaNative), not
// the relay lane — every leg on the code_divergence family. Until
// the lane lands, the calls cross the generic relay and this test is
// RED.
func TestCodeDivergenceWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path   string
		bodies []string
	}{
		{codeDivergenceFindingsPath, codeDivergenceFindingsBodies},
		{codeDivergenceInvestigatePath, codeDivergenceInvestigateBodies},
		{codeDivergenceReportPath, codeDivergenceReportBodies},
	}
	for _, leg := range legs {
		for _, body := range leg.bodies {
			t.Run("POST "+leg.path, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"findings":[]}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, codeDivergenceAuthedPost(leg.path, body, "application/json"))

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
				}
				if core.lastBody != body {
					t.Errorf("upstream body = %q, want the divergence request %q forwarded intact", core.lastBody, body)
				}
				if core.lastPath != leg.path {
					t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, leg.path)
				}
				snap := tel.snapshot()
				lanes, ok := snap.dispatches[codeDivergenceFamily]
				if !ok {
					t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", codeDivergenceFamily)
				}
				if lanes[viaNative] == nil {
					t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", codeDivergenceFamily, lanes)
				}
				if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
					t.Errorf("family %q native core_ok = %d, want 1", codeDivergenceFamily, got)
				}
			})
		}
	}
}

// TestCodeDivergenceSlashSpellingsTerminateNatively pins the
// never-split rule for the wave: every trailing-slash spelling
// terminates on the native lane exactly like its bare spelling.
func TestCodeDivergenceSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	bodies := map[string][]string{
		codeDivergenceFindingsPath:    codeDivergenceFindingsBodies,
		codeDivergenceInvestigatePath: codeDivergenceInvestigateBodies,
		codeDivergenceReportPath:      codeDivergenceReportBodies,
	}
	for _, entry := range codeDivergenceWave {
		t.Run(entry.Method+" "+entry.Path+" slash", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"findings":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := codeDivergenceAuthedPost(entry.Path, bodies[entry.Path][0], "application/json")
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

// TestCodeDivergenceTerminated_ForwardsCallerBytes pins that the
// terminated reader forwards the caller's method, path, query,
// negotiation headers, credential, and body to the same core route:
// exactly one upstream hit, one family authorizer allow, no dest
// retry, no credential widening — for the findings and report legs.
func TestCodeDivergenceTerminated_ForwardsCallerBytes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"findings":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	req := codeDivergenceAuthedPost(codeDivergenceFindingsPath, codeDivergenceFindingsBodies[0], EnvelopeMIME)
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
	if core.lastPath != codeDivergenceFindingsPath {
		t.Errorf("upstream path = %q, want the wave path", core.lastPath)
	}
	if core.lastBody != codeDivergenceFindingsBodies[0] {
		t.Errorf("upstream body = %q, want the caller body intact", core.lastBody)
	}
	if core.lastAccept != EnvelopeMIME {
		t.Errorf("upstream Accept = %q, want the negotiated shape intact", core.lastAccept)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization carries the caller credential forward, got %q", core.lastAuth)
	}
	if got := rec.Header().Get("X-Correlation-ID"); got != "divergence-parity-1" {
		t.Errorf("response correlation = %q, want the caller value echoed", got)
	}

	report := codeDivergenceAuthedPost(codeDivergenceReportPath, codeDivergenceReportBodies[0], "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, report)

	if rec.Code != http.StatusOK {
		t.Fatalf("report status = %d, want 200 from the stub core", rec.Code)
	}
	if core.hits != 2 {
		t.Errorf("upstream hits = %d, want exactly 2 total (one per call, no dest retry)", core.hits)
	}
	if core.lastPath != codeDivergenceReportPath {
		t.Errorf("upstream path = %q, want the report wave path", core.lastPath)
	}

	snap := tel.snapshot()
	if got := snap.authorizer[codeDivergenceFamily][decisionAllow]; got != 2 {
		t.Errorf("authorizer[%s][allow] = %d, want exactly 2 for the terminated calls", codeDivergenceFamily, got)
	}
}
