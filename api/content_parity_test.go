// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Wave-2 Content terminated-vs-relay byte parity (Refs
// eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491, satellite
// eshu-hq/eshu-api#72). The terminated reader must not distort the
// core's answer: for every stub-core outcome its client-visible bytes
// must equal the generic byte-relay's. This guards the dest-owned
// response mapping (forwardToCore on the native lane) against future
// divergence from the relay contract it mirrors.

// contentParityBodies covers every gated wave route for the parity
// matrix. The lines leg carries the pin's slicing-edge bodies (full
// range, sub-range, start clamp, end-zero, end clamp, start-past-end):
// parity must exercise the slicing edges or the split is unproven, so
// the matrix drives those bodies through the native lane and
// TestContentPointReadsBodiesForwarded pins they arrive intact.
var contentParityBodies = []struct {
	path string
	body string
}{
	{"/api/v0/content/files/read", `{"repo_id":"repo-1","relative_path":"src/app.go"}`},
	{"/api/v0/content/files/lines", `{"repo_id":"repo-1","relative_path":"src/app.go","start_line":1,"end_line":5}`},
	{"/api/v0/content/files/lines", `{"repo_id":"repo-1","relative_path":"src/app.go","start_line":2,"end_line":4}`},
	{"/api/v0/content/files/lines", `{"repo_id":"repo-1","relative_path":"src/app.go","start_line":0,"end_line":2}`},
	{"/api/v0/content/files/lines", `{"repo_id":"repo-1","relative_path":"src/app.go","start_line":2,"end_line":0}`},
	{"/api/v0/content/files/lines", `{"repo_id":"repo-1","relative_path":"src/app.go","start_line":4,"end_line":99}`},
	{"/api/v0/content/files/lines", `{"repo_id":"repo-1","relative_path":"src/app.go","start_line":9,"end_line":99}`},
	{"/api/v0/content/entities/read", `{"entity_id":"entity-1"}`},
}

// TestContentPointReadsTerminatedMatchesRelayByteForByte pins that the
// terminated reader relays core bytes exactly like the generic proxy:
// same status, same headers (modulo per-hop framing), same body —
// across the pinned outcome matrix in both negotiated shapes. The
// matrix runs every wave route and slicing-edge body; the stub answers
// the pinned wire strings where core owns the text, and opaque bytes
// where the relay (not the text) is what this test pins.
func TestContentPointReadsTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	outcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"plain 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"repo_id":"repo-1","relative_path":"src/app.go"}`},
		{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repo_id is required"}`},
		{"not found 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"content file not found"}`},
		{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"reader is not configured"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"reader fenced"}`},
		{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/api/v0/auth/oidc/login"}}, ""},
	}
	for _, route := range contentParityBodies {
		for _, outcome := range outcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				name := "POST " + route.path + " " + route.body + " " + outcome.name + " " + accept
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					nativeCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
					nativeMux := http.NewServeMux()
					nativeMux.Handle("/", nativeCore.handler())
					nativeUpstream := httptest.NewServer(nativeMux)

					relayCore := &stubCore{status: outcome.status, header: relayHeaders(outcome.header), body: outcome.body}
					relayMux := http.NewServeMux()
					relayMux.Handle("/", relayCore.handler())
					relayUpstream := httptest.NewServer(relayMux)

					build := func(target, body string) *http.Request {
						req := contentAuthedPost(target, body, accept)
						req.Header.Set("X-Correlation-ID", "content-parity-1")
						return req
					}

					nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
					nativeRec := httptest.NewRecorder()
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, build(route.path, route.body))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					// The relay side crosses the same middleware chain
					// the router builds for the generic proxy
					// (instrumented + key gate), so correlation echo,
					// deprecation headers, and lane filing match the
					// production relay path exactly.
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, build(route.path, route.body))

					nativeRes, relayRes := nativeRec.Result(), relayRec.Result()
					nativeBody, _ := io.ReadAll(nativeRes.Body)
					relayBody, _ := io.ReadAll(relayRes.Body)
					nativeUpstream.Close()
					relayUpstream.Close()
					if nativeRes.StatusCode != relayRes.StatusCode {
						t.Fatalf("native status = %d, relay status = %d, want identical", nativeRes.StatusCode, relayRes.StatusCode)
					}
					if string(nativeBody) != string(relayBody) {
						t.Errorf("native body = %q, relay body = %q, want identical", nativeBody, relayBody)
					}
					for _, header := range []string{"Content-Type", "Retry-After", "Location", "Cache-Control", "X-Correlation-ID"} {
						if nativeRes.Header.Get(header) != relayRes.Header.Get(header) {
							t.Errorf("header %s: native = %q, relay = %q, want identical", header, nativeRes.Header.Get(header), relayRes.Header.Get(header))
						}
					}
					if nativeCore.lastBody != relayCore.lastBody {
						t.Errorf("upstream body: native = %q, relay = %q, want identical", nativeCore.lastBody, relayCore.lastBody)
					}
				})
			}
		}
	}
}

// TestContentPointReadsBodiesForwarded pins that the pin's request
// bodies cross the native lane untouched: the repository and entity
// selectors plus the line-range edges are core-judged, never
// dest-parsed, so the native lane must forward them byte-identical.
func TestContentPointReadsBodiesForwarded(t *testing.T) {
	t.Parallel()

	for _, route := range contentParityBodies {
		t.Run("POST "+route.path+" "+route.body, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"repo_id":"repo-1"}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			rec := httptest.NewRecorder()
			srv.Handler(DefaultV0Sunset).ServeHTTP(rec, contentAuthedPost(route.path, route.body, "application/json"))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			if core.lastBody != route.body {
				t.Errorf("upstream body = %q, want the request body %q forwarded intact", core.lastBody, route.body)
			}
		})
	}
}
