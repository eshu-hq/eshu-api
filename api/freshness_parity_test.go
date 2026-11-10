// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Wave-1 Freshness terminated-vs-relay byte parity (Refs
// eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491, satellite
// eshu-hq/eshu-api#72). The terminated reader must not distort the
// core's answer: for every stub-core outcome its client-visible bytes
// must equal the generic byte-relay's. This guards the dest-owned
// response mapping (forwardToCore on the native lane) against future
// divergence from the relay contract it mirrors.

// freshnessParityRoutes covers every gated wave route for the parity
// matrix. The changed-since legs carry the pin's filter queries
// (scope/repository selectors, since references, sample_limit):
// parity must exercise filter queries or the split is unproven, so
// the matrix drives those queries through the native lane and
// TestFreshnessFilterQueriesForwarded pins they arrive intact.
var freshnessParityRoutes = []string{
	"/api/v0/freshness/changed-since?scope_id=scope_1&since_generation_id=gen_9",
	"/api/v0/freshness/changed-since?repository=repo_1&since_observed_at=2026-11-01T00:00:00Z&sample_limit=10",
	"/api/v0/freshness/generations?scope_id=scope_1&status=active&limit=10",
	"/api/v0/freshness/generations?collector_kind=repo&generation_id=gen_9",
	"/api/v0/freshness/services/changed-since?service_id=svc_1&since_generation_id=gen_9",
	"/api/v0/freshness/services/changed-since?service_id=svc_1&since_generation_id=gen_9&scope_id=scope_1&sample_limit=10",
}

// TestFreshnessTerminatedMatchesRelayByteForByte pins that the
// terminated reader relays core bytes exactly like the generic proxy:
// same status, same headers (modulo per-hop framing), same body —
// across the pinned outcome matrix in both negotiated shapes. The
// matrix runs every wave route; the stub answers the pinned wire
// strings where core owns the text, and opaque bytes where the relay
// (not the text) is what this test pins.
func TestFreshnessTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	outcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"plain 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"data":[],"error":null,"truth":null}`},
		{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid search request"}`},
		{"scope not found 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"scope not found"}`},
		{"ambiguous 409", http.StatusConflict, nil, `{"error":"ambiguous","detail":"two services match"}`},
		{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"reader is not configured"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"reader fenced"}`},
		{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/api/v0/auth/oidc/login"}}, ""},
	}
	for _, route := range freshnessParityRoutes {
		for _, outcome := range outcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				name := "GET " + route + " " + outcome.name + " " + accept
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

					build := func(target string) *http.Request {
						req := authedRequest(http.MethodGet, target)
						req.Header.Set("Accept", accept)
						req.Header.Set("X-Correlation-ID", "freshness-parity-1")
						return req
					}

					nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
					nativeRec := httptest.NewRecorder()
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, build(route))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					// The relay side crosses the same middleware chain
					// the router builds for the generic proxy
					// (instrumented + key gate), so correlation echo,
					// deprecation headers, and lane filing match the
					// production relay path exactly.
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, build(route))

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
					if nativeCore.lastQuery != relayCore.lastQuery {
						t.Errorf("upstream query: native = %q, relay = %q, want identical", nativeCore.lastQuery, relayCore.lastQuery)
					}
				})
			}
		}
	}
}

// TestFreshnessFilterQueriesForwarded pins that the pin's filter
// queries cross the native lane untouched: the scope/repository
// selectors, the since references, and the sample_limit cap are
// core-judged, never dest-parsed, so the native lane must forward
// them byte-identical.
func TestFreshnessFilterQueriesForwarded(t *testing.T) {
	t.Parallel()

	filters := []string{
		"/api/v0/freshness/changed-since?scope_id=scope_1&since_generation_id=gen_9",
		"/api/v0/freshness/changed-since?repository=repo_1&since_observed_at=2026-11-01T00:00:00Z",
		"/api/v0/freshness/changed-since?scope_id=scope_1&since_generation_id=gen_9&sample_limit=10",
		"/api/v0/freshness/generations?scope_id=scope_1&collector_kind=repo&status=active&limit=10",
		"/api/v0/freshness/generations?generation_id=gen_9",
		"/api/v0/freshness/services/changed-since?service_id=svc_1&since_generation_id=gen_9",
		"/api/v0/freshness/services/changed-since?service_id=svc_1&since_generation_id=gen_9&scope_id=scope_1&sample_limit=10",
	}
	for _, target := range filters {
		t.Run("GET "+target, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			rec := httptest.NewRecorder()
			srv.Handler(DefaultV0Sunset).ServeHTTP(rec, authedRequest(http.MethodGet, target))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			want := strings.SplitN(target, "?", 2)[1]
			if core.lastQuery != want {
				t.Errorf("upstream query = %q, want the filter query %q forwarded intact", core.lastQuery, want)
			}
		})
	}
}
