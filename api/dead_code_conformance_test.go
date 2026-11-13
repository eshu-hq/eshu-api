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

// Dead-code relay-leg termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#124). TDD record: the frozen wave fails
// RED before the implementation lands (deadCodeWave undefined), and
// the ahead-of-relay assertions fail next (the 3 dead-code routes still
// cross the generic relay); both are green with the wave gate
// registered ahead of the generic proxy. Until the lane lands, every
// dead-code call crosses the generic byte-relay, so the ahead-of-relay
// assertions fail first.

// TestDeadCodeWaveFrozen pins the exact surface the satellite
// terminates: the 3 dead-code POSTs behind CodeDeadCodeReadAdmission v0
// (the graph run plus the thirteen content reads every leg admits
// through). A wave change moves the ownership note, the fragment
// reference, and this mirror together — the same frozen-set pattern as
// TestDriftWaveFrozen.
func TestDeadCodeWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/code/dead-code code_dead_code gated go/internal/query/openapi/paths/code/dead/scan.go",
		"POST /api/v0/code/dead-code/cross-repo code_dead_code gated go/internal/query/openapi/paths/code/dead/cross_repo.go",
		"POST /api/v0/code/dead-code/investigate code_dead_code gated go/internal/query/openapi/paths/code/dead/investigation.go",
	}
	var got []string
	for _, entry := range deadCodeWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("deadCodeWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("deadCodeWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// Wave dead-code route paths, shared by the behavior suites.
const (
	deadCodeScanPath        = "/api/v0/code/dead-code"
	deadCodeCrossRepoPath   = "/api/v0/code/dead-code/cross-repo"
	deadCodeInvestigatePath = "/api/v0/code/dead-code/investigate"
	deadCodeFamily          = "code_dead_code"
)

// deadCodeScanBodies are the well-formed request bodies the behavior
// tests drive through the scan leg: the corpus-wide shape, the
// repo-scoped shape with paging, and the candidate-kind shape. The
// satellite forwards every shape opaquely; core validates them.
var deadCodeScanBodies = []string{
	`{"language":"go"}`,
	`{"repo_id":"repo-1","language":"go","limit":100}`,
	`{"repo_id":"repo-1","candidate_kind":"Function","limit":10}`,
}

// deadCodeCrossRepoBodies are the well-formed request bodies the
// behavior tests drive through the cross-repo leg: the repo-scoped
// producer shape and the consumer-scoped handles shape.
var deadCodeCrossRepoBodies = []string{
	`{"repo_id":"repo-1","language":"go"}`,
	`{"repo_id":"repo-1","consumer_repo_ids":["repo-2"],"evidence_detail":"handles"}`,
}

// deadCodeInvestigateBodies are the well-formed request bodies the
// behavior tests drive through the investigate leg: the corpus-wide
// shape and the repo-scoped shape.
var deadCodeInvestigateBodies = []string{
	`{"language":"go"}`,
	`{"repo_id":"repo-1","language":"go"}`,
}

// deadCodeAuthedPost builds a shared-key dead-code POST with one of the
// parity bodies, the negotiated shape, and a pinned correlation ID.
func deadCodeAuthedPost(path, body, accept string) *http.Request {
	req := authedRequest(http.MethodPost, path)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "dead-code-parity-1")
	return req
}

// TestDeadCodeWaveAheadOfRelay pins that the wave routes are explicit
// registrations ahead of the generic proxy: a shared-key call must
// file its dispatch on the native lane (viaNative), not the relay
// lane — every leg on the code_dead_code family. Until the lane lands,
// the calls cross the generic relay and this test is RED.
func TestDeadCodeWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path   string
		bodies []string
	}{
		{deadCodeScanPath, deadCodeScanBodies},
		{deadCodeCrossRepoPath, deadCodeCrossRepoBodies},
		{deadCodeInvestigatePath, deadCodeInvestigateBodies},
	}
	for _, leg := range legs {
		for _, body := range leg.bodies {
			t.Run("POST "+leg.path, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"candidates":[]}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, deadCodeAuthedPost(leg.path, body, "application/json"))

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
				}
				if core.lastBody != body {
					t.Errorf("upstream body = %q, want the dead-code request %q forwarded intact", core.lastBody, body)
				}
				if core.lastPath != leg.path {
					t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, leg.path)
				}
				snap := tel.snapshot()
				lanes, ok := snap.dispatches[deadCodeFamily]
				if !ok {
					t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", deadCodeFamily)
				}
				if lanes[viaNative] == nil {
					t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", deadCodeFamily, lanes)
				}
				if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
					t.Errorf("family %q native core_ok = %d, want 1", deadCodeFamily, got)
				}
			})
		}
	}
}

// TestDeadCodeSlashSpellingsTerminateNatively pins the never-split
// rule for the wave: every trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestDeadCodeSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	bodies := map[string][]string{
		deadCodeScanPath:        deadCodeScanBodies,
		deadCodeCrossRepoPath:   deadCodeCrossRepoBodies,
		deadCodeInvestigatePath: deadCodeInvestigateBodies,
	}
	for _, entry := range deadCodeWave {
		t.Run(entry.Method+" "+entry.Path+" slash", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"candidates":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := deadCodeAuthedPost(entry.Path, bodies[entry.Path][0], "application/json")
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

// TestDeadCodeTerminated_ForwardsCallerBytes pins that the terminated
// reader forwards the caller's method, path, query, negotiation
// headers, credential, and body to the same core route: exactly one
// upstream hit, one family authorizer allow, no dest retry, no
// credential widening — for the scan and cross-repo legs.
func TestDeadCodeTerminated_ForwardsCallerBytes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"candidates":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	req := deadCodeAuthedPost(deadCodeScanPath, deadCodeScanBodies[1], EnvelopeMIME)
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
	if core.lastPath != deadCodeScanPath {
		t.Errorf("upstream path = %q, want the wave path", core.lastPath)
	}
	if core.lastBody != deadCodeScanBodies[1] {
		t.Errorf("upstream body = %q, want the caller body intact", core.lastBody)
	}
	if core.lastAccept != EnvelopeMIME {
		t.Errorf("upstream Accept = %q, want the negotiated shape intact", core.lastAccept)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization carries the caller credential forward, got %q", core.lastAuth)
	}
	if got := rec.Header().Get("X-Correlation-ID"); got != "dead-code-parity-1" {
		t.Errorf("response correlation = %q, want the caller value echoed", got)
	}

	cross := deadCodeAuthedPost(deadCodeCrossRepoPath, deadCodeCrossRepoBodies[0], "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, cross)

	if rec.Code != http.StatusOK {
		t.Fatalf("cross-repo status = %d, want 200 from the stub core", rec.Code)
	}
	if core.hits != 2 {
		t.Errorf("upstream hits = %d, want exactly 2 total (one per call, no dest retry)", core.hits)
	}
	if core.lastPath != deadCodeCrossRepoPath {
		t.Errorf("upstream path = %q, want the cross-repo wave path", core.lastPath)
	}

	snap := tel.snapshot()
	if got := snap.authorizer[deadCodeFamily][decisionAllow]; got != 2 {
		t.Errorf("authorizer[%s][allow] = %d, want exactly 2 for the terminated calls", deadCodeFamily, got)
	}
}
