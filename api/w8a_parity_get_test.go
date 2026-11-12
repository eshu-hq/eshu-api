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

// W8a satellite termination GET byte parity (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#100). The terminated reader must not
// distort the core's answer: for every stub-core outcome its
// client-visible bytes must equal the generic byte-relay's. This
// guards the dest-owned response mapping (forwardToCore on the
// native lane) against future divergence from the relay contract it
// mirrors.

// w8aParityGetRoutes covers the 10 wave GETs. Query strings pin the
// filter legs the pins admit: the secrets scoped reads, the
// selectors allowed-scope inventory, and the metrics range — parity
// must exercise those queries through the native lane, so the matrix
// drives them here and TestW8aFilterQueriesForwarded pins they
// arrive intact.
var w8aParityGetRoutes = []string{
	"/api/v0/secrets-iam/identity-trust-chains?scope_id=scope_1",
	"/api/v0/secrets-iam/privilege-posture-observations?scope_id=scope_1",
	"/api/v0/secrets-iam/secret-access-paths?scope_id=scope_1",
	"/api/v0/secrets-iam/posture-gaps?scope_id=scope_1",
	"/api/v0/secrets-iam/posture-summary",
	"/api/v0/status/governance",
	"/api/v0/status/answer-narration",
	"/api/v0/status/hosted-readiness",
	"/api/v0/replatforming/selectors?limit=50",
	"/api/v0/metrics/timeseries?metric=cpu&range=1h",
}

// w8aParityOutcomes is the pinned outcome matrix every W8a route
// carries through both negotiated shapes: the fragment-documented
// outcomes (200, 400, 403, 404, fixed-message 500, retryable 503
// with Retry-After, 504, unfollowed redirect) plus the permanent
// core 503. The route-specific legs (grant sections, allow-list
// rejections, error-shape mappings) ride TestW8aRouteSpecificParity.
var w8aParityOutcomes = []struct {
	name   string
	status int
	header http.Header
	body   string
}{
	{"plain 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"data":[],"error":null,"truth":null}`},
	{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid search request"}`},
	{"core all-scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`},
	{"tenant scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"admin tenant scope is required"}`},
	{"missing 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"repository not found"}`},
	{"fixed-message 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"reader failed"}`},
	{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"reader is not configured"}`},
	{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"reader fenced"}`},
	{"core 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"graph read timed out"}`},
	{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/api/v0/auth/oidc/login"}}, ""},
}

// TestW8aGetTerminatedMatchesRelayByteForByte pins that the
// terminated GET reader relays core bytes exactly like the generic
// proxy: same status, same headers (modulo per-hop framing), same
// body — across the pinned outcome matrix in both negotiated
// shapes.
func TestW8aGetTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	for _, route := range w8aParityGetRoutes {
		for _, outcome := range w8aParityOutcomes {
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
						req.Header.Set("X-Correlation-ID", "w8a-parity-1")
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

// TestW8aFilterQueriesForwarded pins that the scoped filter queries
// reach the core untouched through the native lane: the secrets
// scope reads, the selectors allowed-scope inventory, and the
// metrics range selection.
func TestW8aFilterQueriesForwarded(t *testing.T) {
	t.Parallel()

	filters := []string{
		"/api/v0/secrets-iam/identity-trust-chains?scope_id=scope_1",
		"/api/v0/secrets-iam/posture-summary?scope_id=scope_1",
		"/api/v0/replatforming/selectors?limit=50",
		"/api/v0/metrics/timeseries?metric=cpu&range=1h",
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
