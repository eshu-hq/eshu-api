// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Wave-1b byte-parity matrix (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#101). The stub core answers a documented outcome
// set per route, so the lane proves parity on the outcomes the
// component-extension family can produce without inventing any
// handler behavior dest-side: the reads 200, validation 400s,
// unknown 404s, configuration-state and retryable 503s
// (Retry-After intact), and gateway 504s, in both negotiated
// shapes. (The family-403 deny branch is unreachable pre-D8 and
// stays out of this matrix;
// TestCompextDenyBranchShapePinned pins its shape.)

type compextParityOutcome struct {
	name   string
	status int
	header http.Header
	body   string
}

var compextRetryable503 = http.Header{"Retry-After": []string{"5"}}

func compextOutcomes() []compextParityOutcome {
	return []compextParityOutcome{
		{"reads 200", http.StatusOK, nil, `{"items":[],"truncated":false}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid request"}`},
		{"unknown 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"not found"}`},
		{"unavailable 503 no retry-after", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"backend is unavailable"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, compextRetryable503, `{"error":"Service Unavailable","detail":"backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// TestCompextTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for both
// routes: status, headers, and body cross identically in both
// negotiated shapes.
func TestCompextTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	for _, leg := range compextRedLegs {
		for _, outcome := range compextOutcomes() {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				name := leg.method + " " + leg.path + " " + outcome.name + " " + accept
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					nativeCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
					nativeMux := http.NewServeMux()
					nativeMux.Handle("/", nativeCore.handler())
					nativeUpstream := httptest.NewServer(nativeMux)
					defer nativeUpstream.Close()

					relayCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
					relayMux := http.NewServeMux()
					relayMux.Handle("/", relayCore.handler())
					relayUpstream := httptest.NewServer(relayMux)
					defer relayUpstream.Close()

					nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)

					nativeReq := compextAuthedReq(leg.method, leg.path, leg.body)
					nativeReq.Header.Set("Accept", accept)
					nativeRec := httptest.NewRecorder()
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, nativeReq)
					nativeRes := nativeRec.Result()
					nativeBody, _ := io.ReadAll(nativeRes.Body)

					relayReq := compextAuthedReq(leg.method, leg.path, leg.body)
					relayReq.Header.Set("Accept", accept)
					relayRec := httptest.NewRecorder()
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, relayReq)
					relayRes := relayRec.Result()
					relayBody, _ := io.ReadAll(relayRes.Body)

					if nativeRes.StatusCode != relayRes.StatusCode {
						t.Errorf("status native = %d, relay = %d, want identical", nativeRes.StatusCode, relayRes.StatusCode)
					}
					if string(nativeBody) != string(relayBody) {
						t.Errorf("body native = %q, relay = %q, want identical", nativeBody, relayBody)
					}
					if nativeRes.Header.Get("Retry-After") != relayRes.Header.Get("Retry-After") {
						t.Errorf("Retry-After native = %q, relay = %q, want identical", nativeRes.Header.Get("Retry-After"), relayRes.Header.Get("Retry-After"))
					}
					if nativeRes.StatusCode != outcome.status {
						t.Errorf("status = %d, want stub core %d", nativeRes.StatusCode, outcome.status)
					}
				})
			}
		}
	}
}
