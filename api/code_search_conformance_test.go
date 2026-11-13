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

// Code-search relay-leg termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#113). TDD record: the frozen wave fails
// RED before the implementation lands (codeSearchWave undefined), and
// the ahead-of-relay assertions fail next (the 2 code search POSTs
// still cross the generic relay); both are green with the wave gate
// registered ahead of the generic proxy. Until the lane lands, every
// code search call crosses the generic byte-relay, so the
// ahead-of-relay assertions fail first.

// TestCodeSearchWaveFrozen pins the exact surface the satellite
// terminates: the 2 code search POSTs behind CodeSearchReadAdmission
// v0 (9 re-declared methods: the graph lane plus the global, fallback,
// and corpus content reads for /code/search; the symbol-aware read
// plus the plain-name fallback for /code/symbols/search). A wave
// change moves the ownership note, the fragment reference, and this
// mirror together — the same frozen-set pattern as
// TestCodeFlowWaveFrozen.
func TestCodeSearchWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/code/search code_search gated go/internal/query/openapi/paths/code/routes.go",
		"POST /api/v0/code/symbols/search code_search gated go/internal/query/openapi/paths/code/symbols.go",
	}
	var got []string
	for _, entry := range codeSearchWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("codeSearchWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("codeSearchWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// codeSearchConcretePaths lists one mux match per wave entry. Each
// trailing-slash spelling rides the same native registration
// (never-split), pinned by
// TestCodeSearchSlashSpellingsTerminateNatively.
var codeSearchConcretePaths = []string{
	"/api/v0/code/search",
	"/api/v0/code/symbols/search",
}

// codeSearchParityBodies are the well-formed request bodies the
// behavior tests drive through the lane: repo-selected graph shapes,
// the global-index shape, the language/exact candidate shapes, and the
// fallback shapes for the code leg (driving the graph, global-index,
// candidate, fallback, and enrichment branches core-side; the
// satellite forwards every shape opaquely), plus the symbol-aware and
// plain-name fallback shapes for the symbols leg.
var codeSearchParityBodies = map[string][]string{
	"/api/v0/code/search": {
		`{"query":"render"}`,
		`{"repo_id":"repo-1","query":"render"}`,
		`{"repo_id":"repo-1","query":"render","language":"go","exact":true}`,
		`{"query":"rend","limit":25}`,
	},
	"/api/v0/code/symbols/search": {
		`{"symbol":"Render"}`,
		`{"query":"Render"}`,
		`{"symbol":"Render","repo_id":"repo-1","match_mode":"fuzzy"}`,
	},
}

// codeSearchAuthedPost builds a shared-key code search POST with one
// of the parity bodies, the negotiated shape, and a pinned correlation
// ID.
func codeSearchAuthedPost(path, body, accept string) *http.Request {
	req := authedRequest(http.MethodPost, path)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "code-search-parity-1")
	return req
}

// TestCodeSearchWaveAheadOfRelay pins that the wave routes are
// explicit registrations ahead of the generic proxy: a shared-key call
// must file its dispatch on the native lane (viaNative), not the relay
// lane. Until the lane lands, the calls cross the generic relay and
// this test is RED.
func TestCodeSearchWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, path := range codeSearchConcretePaths {
		for _, body := range codeSearchParityBodies[path] {
			t.Run("POST "+path+" "+body, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"results":[],"truth":{"coverage":"exact"}}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, codeSearchAuthedPost(path, body, "application/json"))

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
				}
				if core.lastBody != body {
					t.Errorf("upstream body = %q, want the code search request %q forwarded intact", core.lastBody, body)
				}
				if core.lastPath != path {
					t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, path)
				}
				snap := tel.snapshot()
				lanes, ok := snap.dispatches["code_search"]
				if !ok {
					t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", "code_search")
				}
				if lanes[viaNative] == nil {
					t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", "code_search", lanes)
				}
				if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
					t.Errorf("family %q native core_ok = %d, want 1", "code_search", got)
				}
			})
		}
	}
}

// TestCodeSearchSlashSpellingsTerminateNatively pins the never-split
// rule for the wave: each trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestCodeSearchSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for _, path := range codeSearchConcretePaths {
		t.Run("POST "+path+"/", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"results":[],"truth":{"coverage":"exact"}}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeSearchAuthedPost(path+"/", codeSearchParityBodies[path][0], "application/json"))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.dispatches["code_search"][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", "code_search", got)
			}
		})
	}
}

// TestCodeSearchTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for every
// terminated route: status, headers, and body cross identically in
// both negotiated shapes. The stub core answers each documented search
// outcome (200 found and empty, 400 validation and malformed, core
// 403 grant-denied, fixed-message 500, store-unavailable 503,
// graph-deadline 504, retryable 503 with Retry-After, unfollowed
// redirect), so the lane proves parity on the error body/code/message
// surface the arbiter bar names without inventing any handler behavior
// dest-side.
func TestCodeSearchTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	outcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"plain 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"results":[],"truth":{"coverage":"exact"}}`},
		{"empty 200", http.StatusOK, nil, `{"results":[],"count":0}`},
		{"validation 400 short-query", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"query must be at least 3 characters"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unexpected end of JSON input"}`},
		{"core grant 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`},
		{"fixed-message 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"code search read failed"}`},
		{"store unavailable 503", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"code search store is unavailable"}`},
		{"graph deadline 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"graph read exceeded its deadline"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/api/v0/auth/oidc/login"}}, ""},
	}
	for _, path := range codeSearchConcretePaths {
		for _, outcome := range outcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				for _, body := range codeSearchParityBodies[path] {
					name := "POST " + path + " " + outcome.name + " " + accept
					t.Run(name, func(t *testing.T) {
						t.Parallel()

						nativeCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
						nativeMux := http.NewServeMux()
						nativeMux.Handle("/", nativeCore.handler())
						nativeUpstream := httptest.NewServer(nativeMux)
						defer nativeUpstream.Close()

						relayCore := &stubCore{status: outcome.status, header: relayHeaders(outcome.header), body: outcome.body}
						relayMux := http.NewServeMux()
						relayMux.Handle("/", relayCore.handler())
						relayUpstream := httptest.NewServer(relayMux)
						defer relayUpstream.Close()

						nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
						nativeRec := httptest.NewRecorder()
						nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, codeSearchAuthedPost(path, body, accept))

						relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
						relayRec := httptest.NewRecorder()
						relayReq := codeSearchAuthedPost(path, body, accept)
						relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, relayReq)

						nativeRes, relayRes := nativeRec.Result(), relayRec.Result()
						if nativeRes.StatusCode != relayRes.StatusCode {
							t.Fatalf("native status = %d, relay status = %d, want identical", nativeRes.StatusCode, relayRes.StatusCode)
						}
						nativeBody, _ := io.ReadAll(nativeRes.Body)
						relayBody, _ := io.ReadAll(relayRes.Body)
						if string(nativeBody) != string(relayBody) {
							t.Errorf("native body = %q, relay body = %q, want identical", nativeBody, relayBody)
						}
						for _, header := range []string{"Content-Type", "Retry-After", "Location", "Cache-Control", "X-Correlation-ID"} {
							if nativeRes.Header.Get(header) != relayRes.Header.Get(header) {
								t.Errorf("header %s: native = %q, relay = %q, want identical", header, nativeRes.Header.Get(header), relayRes.Header.Get(header))
							}
						}
					})
				}
			}
		}
	}
}
