// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// G2 terminated-vs-relay byte parity (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#101 RE:G2V2). The terminated reader
// must not distort the core's answer: for every stub-core outcome
// its client-visible bytes must equal the generic byte-relay's. This
// guards the dest-owned response mapping (forwardToCore on the
// native lane) against future divergence from the relay contract it
// mirrors.

// g2ParityLegs covers every gated wave route for the parity matrix.
// The wildcard legs drive one concrete instance each; the derive leg
// drives its POST body. The probe query rides every leg to prove the
// native lane forwards the raw query untouched — it names no pin
// filter, it only exercises the opaque-forward path. The POST body
// parity (lastBody equality) pins the derive payload crosses
// intact, never parsed dest-side.
var g2ParityLegs = []struct {
	method string
	path   string
	body   string
}{
	{http.MethodGet, "/api/v0/collector-extraction-readiness?probe=1", ""},
	{http.MethodGet, "/api/v0/collector-extraction-readiness/repo?probe=1", ""},
	{http.MethodGet, "/api/v0/fact-schema-versions?probe=1", ""},
	{http.MethodGet, "/api/v0/fact-schema-versions/file?probe=1", ""},
	{http.MethodGet, "/api/v0/surface-inventory?probe=1", ""},
	{http.MethodPost, "/api/v0/visualizations/derive?probe=1", `{"packet":"derive-probe"}`},
	{http.MethodGet, "/api/v0/repository/manifest-dependencies?probe=1", ""},
}

// TestG2TerminatedMatchesRelayByteForByte pins that the terminated
// reader relays core bytes exactly like the generic proxy: same
// status, same headers (modulo per-hop framing), same body — across
// the pinned outcome matrix in both negotiated shapes. The matrix
// runs every wave route; the stub answers the pinned wire strings
// where core owns the text, and opaque bytes where the relay (not
// the text) is what this test pins.
func TestG2TerminatedMatchesRelayByteForByte(t *testing.T) {
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
	for _, leg := range g2ParityLegs {
		for _, outcome := range outcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				name := leg.method + " " + leg.path + " " + outcome.name + " " + accept
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
						req := g2AuthedReq(leg.method, target, leg.body)
						req.Header.Set("Accept", accept)
						req.Header.Set("X-Correlation-ID", "g2-parity-1")
						return req
					}

					nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
					nativeRec := httptest.NewRecorder()
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, build(leg.path))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					// The relay side crosses the same middleware chain
					// the router builds for the generic proxy
					// (instrumented + key gate), so correlation echo,
					// deprecation headers, and lane filing match the
					// production relay path exactly.
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, build(leg.path))

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
					if nativeCore.lastBody != relayCore.lastBody {
						t.Errorf("upstream body: native = %q, relay = %q, want identical", nativeCore.lastBody, relayCore.lastBody)
					}
				})
			}
		}
	}
}
