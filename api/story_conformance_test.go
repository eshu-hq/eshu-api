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

// Repository story relay-leg termination conformance (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97). TDD record: the
// frozen wave fails RED before the implementation lands (storyWave
// undefined), and the ahead-of-relay assertion fails next (the story
// GET still rides the explicit D4 relay leg); both are green with the
// wave gate registered ahead of the generic proxy. Until the lane
// lands, the story call relays, so the ahead-of-relay assertion fails
// first.

// TestStoryWaveFrozen pins the exact surface the satellite terminates:
// the W5f story single behind RepositoryStoryReadAdmission v0. A wave
// change moves the ownership note, the fragment reference, and this
// mirror together.
func TestStoryWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/repositories/{repo_id}/story repositories gated go/internal/query/openapi/paths/repository/routes.go",
	}
	var got []string
	for _, entry := range storyWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("storyWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("storyWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// storyConcretePath is the concrete story path the behavior tests
// drive (wildcard resolved to a fixed test value).
const storyConcretePath = "/api/v0/repositories/repo_1/story"

// TestStoryWaveAheadOfRelay pins that the story leg is an explicit
// registration ahead of the generic proxy: a shared-key call must
// file its dispatch on the native lane (viaNative), not the relay
// lane — superseding the carried D4 relay leg
// (TestD4StoryRelayLegOnGenericProxy, retired with this lane). Until
// the lane lands, the call relays and this test is RED.
func TestStoryWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, storyConcretePath)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Correlation-ID", "story-parity-1")

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
	lanes, ok := snap.dispatches["repositories"]
	if !ok {
		t.Fatalf("family repositories has no dispatch rows, want the explicit wave route ahead of relay")
	}
	if lanes[viaNative] == nil {
		t.Errorf("family repositories dispatches = %v, want a native lane row for the shared-key wave call", lanes)
	}
	if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
		t.Errorf("family repositories native core_ok = %d, want 1", got)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (one server-side fetch, no retry)", core.hits)
	}
	if core.lastPath != storyConcretePath {
		t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, storyConcretePath)
	}
}
