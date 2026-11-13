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

// W4 Content search relay-leg termination conformance (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#111). TDD record: the
// frozen wave failed RED before the implementation landed
// (contentSearchWave undefined), and the ahead-of-relay assertions
// failed next (the 4 search POSTs still crossed the generic relay);
// both are green with the wave gate registered ahead of the generic
// proxy. Until the lane lands, every search call crosses the generic
// byte-relay, so the ahead-of-relay assertions fail first.

// TestContentSearchWaveFrozen pins the exact surface the satellite
// terminates: the 4 content search POSTs behind
// ContentSearchReadAdmission v0 (10 re-declared methods covering the
// files/entities scoped, fallback, corpus, and unscoped branches plus
// the dockerfile-install and php-config entity pages). A wave change
// moves the ownership note, the fragment reference, and this mirror
// together — the same frozen-set pattern as TestCodeFlowWaveFrozen.
func TestContentSearchWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/content/dockerfile-installs/search content gated go/internal/query/openapi/paths/search/content.go",
		"POST /api/v0/content/entities/search content gated go/internal/query/openapi/paths/search/content.go",
		"POST /api/v0/content/files/search content gated go/internal/query/openapi/paths/search/content.go",
		"POST /api/v0/content/php-config/search content gated go/internal/query/openapi/paths/search/content.go",
	}
	var got []string
	for _, entry := range contentSearchWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("contentSearchWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("contentSearchWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// contentSearchConcretePaths lists one mux match per wave entry. Each
// trailing-slash spelling rides the same native registration
// (never-split), pinned by
// TestContentSearchSlashSpellingsTerminateNatively.
var contentSearchConcretePaths = []string{
	"/api/v0/content/files/search",
	"/api/v0/content/entities/search",
	"/api/v0/content/dockerfile-installs/search",
	"/api/v0/content/php-config/search",
}

// contentSearchParityBodies are the well-formed request bodies the
// behavior tests drive through the lane: scoped, multi-repo, and
// unscoped shapes for the files/entities legs (driving the paged-seam,
// fallback, corpus, and bounded-walk branches core-side; the satellite
// forwards every shape opaquely), the package selector shapes for the
// dockerfile-install leg, and the directive/mode shapes for the
// php-config leg.
var contentSearchParityBodies = map[string][]string{
	"/api/v0/content/files/search": {
		`{"query":"render"}`,
		`{"repo_id":"repo-1","query":"render"}`,
		`{"repo_ids":["repo-1","repo-2"],"query":"render"}`,
		`{"pattern":"render","repo_id":"repo-1"}`,
	},
	"/api/v0/content/entities/search": {
		`{"query":"render"}`,
		`{"repo_id":"repo-1","query":"render"}`,
		`{"pattern":"render"}`,
	},
	"/api/v0/content/dockerfile-installs/search": {
		`{"package":"curl"}`,
		`{"package":"curl","repo_id":"repo-1"}`,
	},
	"/api/v0/content/php-config/search": {
		`{"directive":"memory_limit"}`,
		`{"mode":"php-fpm"}`,
	},
}

// TestContentSearchWaveAheadOfRelay pins that every wave route is an
// explicit registration ahead of the generic proxy: a shared-key call
// must file its dispatch on the native lane (viaNative), not the relay
// lane, with the request body forwarded intact through the real mux.
// Until the lane lands, the calls cross the generic relay and this
// test is RED.
func TestContentSearchWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, path := range contentSearchConcretePaths {
		for _, body := range contentSearchParityBodies[path] {
			t.Run("POST "+path+" "+body, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"results":[],"count":0}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, contentAuthedPost(path, body, "application/json"))

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
				}
				if core.lastBody != body {
					t.Errorf("upstream body = %q, want the search request %q forwarded intact", core.lastBody, body)
				}
				if core.lastPath != path {
					t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, path)
				}
				snap := tel.snapshot()
				lanes, ok := snap.dispatches["content"]
				if !ok {
					t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", "content")
				}
				if lanes[viaNative] == nil {
					t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", "content", lanes)
				}
				if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
					t.Errorf("family %q native core_ok = %d, want 1", "content", got)
				}
			})
		}
	}
}

// TestContentSearchSlashSpellingsTerminateNatively pins the never-split
// rule for the wave: every trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestContentSearchSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for _, path := range contentSearchConcretePaths {
		t.Run("POST "+path+"/", func(t *testing.T) {
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
			handler.ServeHTTP(rec, contentAuthedPost(path+"/", contentSearchParityBodies[path][0], "application/json"))

			if rec.Code != http.StatusOK {
				t.Fatalf("POST %s/: status = %d, want 200 from the stub core", path, rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.dispatches["content"][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", "content", got)
			}
		})
	}
}

// TestContentSearchTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for every
// terminated route and every driven body shape: status, headers, and
// body cross identically in both negotiated shapes. The stub core
// answers each documented search outcome (200 found, 200 empty, 400
// malformed/validation, 403 grant-denied, 404, fixed-message 500,
// store-unavailable 503, retryable 503 with Retry-After, 504
// deadline, unfollowed redirect), so the lane proves parity on the
// error body/code/message surface without inventing handler behavior
// dest-side.
func TestContentSearchTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	outcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"found 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"results":[{"repo_id":"repo-1"}],"count":1}`},
		{"empty 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"results":[],"count":0}`},
		{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"query is required"}`},
		{"core grant 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"grant denied"}`},
		{"missing 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"no match"}`},
		{"fixed-message 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"search failed"}`},
		{"store unavailable 503", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"search store is unavailable"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"search backend is unavailable"}`},
		{"deadline 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"search exceeded its deadline"}`},
		{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/api/v0/auth/oidc/login"}}, ""},
	}
	for _, path := range contentSearchConcretePaths {
		for _, body := range contentSearchParityBodies[path] {
			for _, outcome := range outcomes {
				for _, accept := range []string{EnvelopeMIME, "application/json"} {
					name := "POST " + path + " " + body + " " + outcome.name + " " + accept
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
						nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, contentAuthedPost(path, body, accept))

						relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
						relayRec := httptest.NewRecorder()
						relayReq := contentAuthedPost(path, body, accept)
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
						for _, field := range []struct {
							name   string
							native string
							relay  string
						}{
							{"method", nativeCore.lastMethod, relayCore.lastMethod},
							{"path", nativeCore.lastPath, relayCore.lastPath},
							{"query", nativeCore.lastQuery, relayCore.lastQuery},
							{"body", nativeCore.lastBody, relayCore.lastBody},
						} {
							if field.native != field.relay {
								t.Errorf("upstream %s: native = %q, relay = %q, want identical", field.name, field.native, field.relay)
							}
						}
					})
				}
			}
		}
	}
}
