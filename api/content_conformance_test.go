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

// Wave-2 Content point-reads conformance (Refs eshu-hq/eshu#9669, D13
// tracker eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#72). TDD
// record: this suite failed RED before the implementation landed
// (frozen wave asserted first; the swap-mutant legs relayed, so the
// ahead-of-relay assertions failed; the unknown route rode a leg, so
// the fail-closed assertion failed) and is green with it — the frozen
// wave below mirrors the ownership-map Content point-read rows behind
// ContentPointReadAdmission v0, and the behavior tests require every
// wave route to terminate natively for shared-key callers while
// unresolvable identity classes relay explicitly.

// TestContentPointReadsWaveFrozen pins the exact Content surface the
// satellite must terminate. A wave change (route added, removed,
// renamed, or rehomed) fails here until the ownership map, the spec
// fragment, and this mirror move together — the same frozen-set
// pattern as TestFreshnessWaveFrozen. The want table is the
// ownership-map rows (method, path, family) plus the satellite serve
// kind and the Decision 5(c) fragment that carries the route's spec
// entry. Still-relayed content legs (the file/entity search legs, the
// list legs, the scoped-entity leg) are NOT in this table — the pin
// seals them out; contracts/content-point-reads-wave.md discloses
// each with its reason.
func TestContentPointReadsWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/content/entities/read content gated paths/search/content_point_reads.go",
		"POST /api/v0/content/files/lines content gated paths/search/content_point_reads.go",
		"POST /api/v0/content/files/read content gated paths/search/content_point_reads.go",
	}
	var got []string
	for _, entry := range contentWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("contentWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("contentWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// contentConcretePaths fills the wave's path set so the behavior
// tests drive real mux matches. Trailing-slash spellings ride the same
// native registration (never-split), so each entry also names its slash
// variant where the mux distinguishes it. The wave carries no path
// wildcards, so every entry maps to itself.
var contentConcretePaths = map[string]string{
	"POST /api/v0/content/files/read":    "/api/v0/content/files/read",
	"POST /api/v0/content/files/lines":   "/api/v0/content/files/lines",
	"POST /api/v0/content/entities/read": "/api/v0/content/entities/read",
}

// contentReadBodies are the well-formed request bodies the behavior
// tests drive through the lane: one per wave route, naming the keys
// the served fragment requires. The lines bodies cover the pin's
// slicing edges (full range, sub-range, start clamp, end-zero, end
// clamp, start-past-end) so the ahead-of-relay proof drives the edge
// the pin shape requires.
var contentReadBodies = map[string][]string{
	"POST /api/v0/content/files/read": {
		`{"repo_id":"repo-1","relative_path":"src/app.go"}`,
	},
	"POST /api/v0/content/files/lines": {
		`{"repo_id":"repo-1","relative_path":"src/app.go","start_line":1,"end_line":5}`,
		`{"repo_id":"repo-1","relative_path":"src/app.go","start_line":2,"end_line":4}`,
		`{"repo_id":"repo-1","relative_path":"src/app.go","start_line":0,"end_line":2}`,
		`{"repo_id":"repo-1","relative_path":"src/app.go","start_line":2,"end_line":0}`,
		`{"repo_id":"repo-1","relative_path":"src/app.go","start_line":4,"end_line":99}`,
		`{"repo_id":"repo-1","relative_path":"src/app.go","start_line":9,"end_line":99}`,
	},
	"POST /api/v0/content/entities/read": {
		`{"entity_id":"entity-1"}`,
	},
}

// contentAuthedPost builds a shared-key Content POST with one parity
// body, the negotiated shape, and a pinned correlation ID.
func contentAuthedPost(path, body, accept string) *http.Request {
	req := authedRequest(http.MethodPost, path)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "content-parity-1")
	return req
}

// TestContentPointReadsWaveAheadOfRelay pins that every wave route is
// an explicit registration ahead of the generic proxy: a shared-key
// call must file its dispatch on the native lane (viaNative), not the
// relay lane, with the request body forwarded intact. This test is the
// swap-mutant detector for dispatch: if the registration (or the
// terminated handler behind it) is swapped back to the generic
// dispatch, every leg files relay and this test goes RED. Until the
// lane lands, every call crosses the generic byte-relay and this test
// is RED.
func TestContentPointReadsWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, entry := range contentWave {
		for _, body := range contentReadBodies[entry.Method+" "+entry.Path] {
			t.Run(entry.Method+" "+entry.Path+" "+body, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"repo_id":"repo-1"}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				key := entry.Method + " " + entry.Path
				target, ok := contentConcretePaths[key]
				if !ok {
					t.Fatalf("no concrete path for wave route %q", key)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, contentAuthedPost(target, body, "application/json"))

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
				}
				if core.lastBody != body {
					t.Errorf("upstream body = %q, want the content request %q forwarded intact", core.lastBody, body)
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
}

// TestContentPointReadsUnknownRouteFailClosed pins the
// terminated-handler switch default: any route the wave table does not
// know answers 404 and never rides a leg. Routes() only passes
// contentWave entries, so this guards the future table-add-without-a-handler
// mistake.
func TestContentPointReadsUnknownRouteFailClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, route := range []string{
		"POST /api/v0/content/files/bogus",
		"POST /api/v0/content/files/read/extra",
		"GET /api/v0/content/files/read",
		"",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v0/content/files/read", nil)
		srv.contentTerminatedHandler(route)(rec, req)

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

// TestContentPointReadsOutOfPinLegsNotInWave pins the wave boundary:
// the legs the point pin seals out never enter the point-read wave.
// The 4 search legs now terminate natively under their own W4 wave
// (Refs eshu-hq/eshu-api#111); the /content family prefix must not
// hijack them into the terminated point table. Each search leg still
// resolves to the content family and now files its dispatch on the
// native lane for a shared-key caller.
func TestContentPointReadsOutOfPinLegsNotInWave(t *testing.T) {
	t.Parallel()

	others := []struct {
		path string
		body string
	}{
		{"/api/v0/content/files/search", `{"query":"render"}`},
		{"/api/v0/content/entities/search", `{"query":"render"}`},
		{"/api/v0/content/dockerfile-installs/search", `{"package":"curl"}`},
		{"/api/v0/content/php-config/search", `{"directive":"memory_limit"}`},
	}
	for _, tc := range others {
		if got := classifyFamily(tc.path); got != "content" {
			t.Errorf("classifyFamily(%q) = %q, want the content family label", tc.path, got)
		}
		for _, entry := range contentWave {
			if entry.Method+" "+entry.Path == http.MethodPost+" "+tc.path {
				t.Errorf("search leg %q is in contentWave, want it in the W4 search wave only", tc.path)
			}
		}
		found := false
		for _, entry := range contentSearchWave {
			if entry.Method+" "+entry.Path == http.MethodPost+" "+tc.path {
				found = true
			}
		}
		if !found {
			t.Errorf("search leg %q is in no wave table, want it in contentSearchWave", tc.path)
		}
	}

	core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range others {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, contentAuthedPost(tc.path, tc.body, "application/json"))
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s: status = %d, want 200 from the stub core", tc.path, rec.Code)
		}
	}
	snap := tel.snapshot()
	if lanes := snap.dispatches["content"]; lanes[viaNative] == nil {
		t.Errorf("content dispatches = %v, want a native lane row for the terminated search legs", snap.dispatches["content"])
	}
	if lanes := snap.dispatches["content"]; lanes[viaRelay] != nil {
		t.Errorf("content dispatches = %v, want no relay lane row while only search legs ran", snap.dispatches["content"])
	}
}

// TestContentPointReadsSlashSpellingsTerminateNatively pins the
// never-split rule for the wave: every trailing-slash spelling
// terminates on the native lane exactly like its bare spelling.
func TestContentPointReadsSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for key, concrete := range contentConcretePaths {
		for _, body := range contentReadBodies[key] {
			t.Run(key+" slash "+body, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"repo_id":"repo-1"}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, contentAuthedPost(concrete+"/", body, "application/json"))

				if rec.Code != http.StatusOK {
					t.Fatalf("POST %s/: status = %d, want 200 from the stub core", concrete, rec.Code)
				}
				snap := tel.snapshot()
				family := classifyFamily(concrete)
				if got := snap.dispatches[family][viaNative][outcomeCoreOK]; got != 1 {
					t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", family, got)
				}
			})
		}
	}
}
