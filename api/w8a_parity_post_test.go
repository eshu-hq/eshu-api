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

// W8a satellite termination POST byte parity (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#100). The terminated reader must not
// distort the core's answer: for every stub-core outcome its
// client-visible bytes must equal the generic byte-relay's, with the
// caller body forwarded intact (POST bodies cross as opaque bytes).

// TestW8aPostTerminatedMatchesRelayByteForByte pins that the
// terminated POST reader relays core bytes exactly like the generic
// proxy: same status, same headers (modulo per-hop framing), same
// body — across the pinned outcome matrix in both negotiated
// shapes — with the wave body arriving intact.
func TestW8aPostTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	for path, body := range w8aPostBodies {
		for _, outcome := range w8aParityOutcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
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

					build := func(target string) *http.Request {
						req := w8aAuthedPost(target, accept)
						return req
					}

					nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
					nativeRec := httptest.NewRecorder()
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, build(path))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					// The relay side crosses the same middleware chain
					// the router builds for the generic proxy
					// (instrumented + key gate), so correlation echo,
					// deprecation headers, and lane filing match the
					// production relay path exactly.
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, build(path))

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
					if nativeCore.lastBody != body {
						t.Errorf("native upstream body = %q, want the wave request %q forwarded intact", nativeCore.lastBody, body)
					}
					if nativeCore.lastBody != relayCore.lastBody {
						t.Errorf("upstream body: native = %q, relay = %q, want identical", nativeCore.lastBody, relayCore.lastBody)
					}
				})
			}
		}
	}
}

// TestW8aRouteSpecificParity pins the route-specific legs the
// arbiter parity bar names: the terminated lane must carry each
// family's documented shapes byte-identically. The stub core emits
// the documented wire shapes; the test proves the native lane
// relays them untouched in both negotiated envelopes.
//
//   - posture-summary with the S3 grant section present AND the
//     graph-read-error mapping (503/504): the OUT grant leg blends
//     core-side on the forwarded call, so the terminated lane must
//     carry the blended section without distortion.
//   - semantic pending_search_vector truth-freshness cause: the
//     freshness cause rides the 200 payload.
//   - terraform FindingFilter double guard (zero-row page) AND
//     capability gating (403): the empty AllowedScopeIDs page and
//     the gate denial cross intact.
//   - metrics allow-list rejection (403) AND the time-series page.
//   - IaC scoped-empty zero-page short-circuits (aws findings,
//     selectors): the empty pages cross intact.
//   - status governance Summary error shape (503 read-failed) AND
//     the observed summary page.
//   - status hosted-readiness repository-count leg AND the
//     non-Neo4j-backend unknown leg.
//   - status answer-narration posture override on AND off.
func TestW8aRouteSpecificParity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method string
		path   string
		body   string
		status int
		header http.Header
		emit   string
	}{
		{
			http.MethodGet, "/api/v0/secrets-iam/posture-summary", "", http.StatusOK, nil,
			`{"data":{"chains":[],"observations":[],"paths":[],"gaps":[],"summary":{"state":"exact"},"grant_posture":{"external_principals":[]}},"error":null,"truth":{}}`,
		},
		{
			http.MethodGet, "/api/v0/secrets-iam/posture-summary", "", http.StatusServiceUnavailable, nil,
			`{"error":"Service Unavailable","detail":"graph grant read failed"}`,
		},
		{
			http.MethodGet, "/api/v0/secrets-iam/posture-summary", "", http.StatusGatewayTimeout, nil,
			`{"error":"Gateway Timeout","detail":"graph grant read timed out"}`,
		},
		{
			http.MethodPost, "/api/v0/search/semantic", `{"query":"drift findings","limit":10}`, http.StatusOK, nil,
			`{"data":[{"score":0.9}],"error":null,"truth":{"pending_search_vector":"indexing"}}`,
		},
		{
			http.MethodPost, "/api/v0/terraform/config-state-drift/findings", `{"scope_id":"scope_1","limit":50}`, http.StatusOK, nil,
			`{"data":[],"error":null,"truth":{"finding_filter":"scoped+empty-allowed-scopes"}}`,
		},
		{
			http.MethodPost, "/api/v0/terraform/config-state-drift/findings", `{"scope_id":"scope_1","limit":50}`, http.StatusForbidden, nil,
			`{"error":"Forbidden","detail":"terraform drift capability is required"}`,
		},
		{
			http.MethodGet, "/api/v0/metrics/timeseries?metric=cpu&range=1h", "", http.StatusOK, nil,
			`{"data":[{"t":1,"v":2}],"error":null,"truth":{}}`,
		},
		{
			http.MethodGet, "/api/v0/metrics/timeseries?metric=nope&range=1h", "", http.StatusForbidden, nil,
			`{"error":"Forbidden","detail":"metric is not on the allow-list"}`,
		},
		{
			http.MethodPost, "/api/v0/aws/runtime-drift/findings", `{"scope_id":"scope_1","limit":50}`, http.StatusOK, nil,
			`{"data":[],"error":null,"truth":{"page":"scoped-empty"}}`,
		},
		{
			http.MethodGet, "/api/v0/replatforming/selectors?limit=50", "", http.StatusOK, nil,
			`{"data":[],"error":null,"truth":{"page":"scoped-empty"}}`,
		},
		{
			http.MethodGet, "/api/v0/status/governance", "", http.StatusOK, nil,
			`{"data":{"audit_state":"observed"},"error":null,"truth":{}}`,
		},
		{
			http.MethodGet, "/api/v0/status/governance", "", http.StatusServiceUnavailable, nil,
			`{"error":"Service Unavailable","detail":"governance audit summary read failed"}`,
		},
		{
			http.MethodGet, "/api/v0/status/hosted-readiness", "", http.StatusOK, nil,
			`{"data":{"repositories":3},"error":null,"truth":{}}`,
		},
		{
			http.MethodGet, "/api/v0/status/hosted-readiness", "", http.StatusOK, nil,
			`{"data":{"state":"unknown","backend":"non-neo4j"},"error":null,"truth":{}}`,
		},
		{
			http.MethodGet, "/api/v0/status/answer-narration", "", http.StatusOK, nil,
			`{"data":{"state":"ready","override":true},"error":null,"truth":{}}`,
		},
		{
			http.MethodGet, "/api/v0/status/answer-narration", "", http.StatusOK, nil,
			`{"data":{"state":"ready","override":false},"error":null,"truth":{}}`,
		},
	}
	for _, tc := range cases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := tc.method + " " + tc.path + " " + accept + "emit"
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: tc.status, header: tc.header, body: tc.emit}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, skeletonTestKey)
				handler := srv.Handler(DefaultV0Sunset)

				var req *http.Request
				if tc.method == http.MethodPost {
					req = authedRequest(http.MethodPost, tc.path)
					req.Body = io.NopCloser(strings.NewReader(tc.body))
					req.ContentLength = int64(len(tc.body))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req = authedRequest(http.MethodGet, tc.path)
				}
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != tc.status {
					t.Fatalf("status = %d, want the emitted core %d through the native lane", rec.Code, tc.status)
				}
				got, _ := io.ReadAll(rec.Result().Body)
				if string(got) != tc.emit {
					t.Errorf("native body = %q, want the emitted core bytes %q carried intact", got, tc.emit)
				}
				if core.hits != 1 {
					t.Errorf("upstream hits = %d, want exactly 1", core.hits)
				}
			})
		}
	}
}
