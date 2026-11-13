// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Divergence byte-parity matrix (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#125). The stub core answers every documented
// divergence outcome per route, so the lane proves parity on the full
// outcome set the arbiter bar names without inventing any handler
// behavior dest-side: the findings page and truncated-page shapes,
// the investigate finding plus next-steps shape with its 404, and the
// report rollup shape — plus validation/malformed 400s,
// ungranted-selector 400s, live 403s, capability 501s, fixed-message
// 500s, retryable 503s with Retry-After, and gateway 504s, in both
// negotiated shapes.

type divergenceParityOutcome struct {
	name   string
	status int
	header http.Header
	body   string
}

// divergenceRetryable503 is the shared retryable-core shape: the lane
// relays it with its Retry-After intact, never converting it to a
// local 503.
var divergenceRetryable503 = http.Header{"Retry-After": []string{"5"}}

// divergenceFindingsOutcomes pins the documented findings answers the
// matrix drives: 200 findings and scoped-empty zero pages, the
// truncated-page shape with its cursor, the selector and validation
// 400s, the live 403, the 501 capability, the fixed-message 500, the
// retryable 503, and the gateway 504.
func divergenceFindingsOutcomes() []divergenceParityOutcome {
	return []divergenceParityOutcome{
		{"findings 200", http.StatusOK, nil, `{"repo_id":"repo-1","kind":"","limit":25,"offset":0,"findings":[{"finding_id":"exact:fp-1","kind":"exact","score":42}],"count":1,"truncated":false,"next_offset":null,"suppressions":{},"source_backend":"postgres_content_store"}`},
		{"scoped-empty zero page 200", http.StatusOK, nil, `{"repo_id":"repo-1","kind":"exact","limit":25,"offset":0,"findings":[],"count":0,"truncated":false,"next_offset":null,"suppressions":{},"source_backend":"postgres_content_store"}`},
		{"truncated 200 stat window left behind", http.StatusOK, nil, `{"repo_id":"repo-1","kind":"","limit":1,"offset":0,"findings":[],"count":0,"truncated":true,"next_offset":1,"suppressions":{"test_suppressed":2},"source_backend":"postgres_content_store+graph"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unexpected end of JSON input"}`},
		{"validation 400 kind unsupported", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"kind must be one of: \"\", \"exact\", \"renamed\", \"drifted\", \"wrapper_bypass\", \"convention_outlier\""}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"code divergence findings require a supported query profile"}`},
		{"fixed-message 500 findings failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"divergence findings read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, divergenceRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// divergenceInvestigateOutcomes pins the documented investigate answers:
// 200 finding-plus-steps and 404 not-found shapes, the validation and
// ungranted selector 400s, the live 403, the 501 capability, the
// fixed-message 500, the retryable 503, and the gateway 504.
func divergenceInvestigateOutcomes() []divergenceParityOutcome {
	return []divergenceParityOutcome{
		{"finding 200", http.StatusOK, nil, `{"repo_id":"repo-1","finding":{"finding_id":"exact:fp-1","kind":"exact"},"next_steps":[{"tool":"call-chain","args":{},"purpose":"follow-up"}],"truncated":false,"suppressions":{},"source_backend":"postgres_content_store"}`},
		{"not-found 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"divergence finding not found"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unexpected end of JSON input"}`},
		{"validation 400 fingerprint required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"fingerprint is required"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"code divergence findings require a supported query profile"}`},
		{"fixed-message 500 investigation failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"divergence investigation read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, divergenceRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// divergenceReportOutcomes pins the documented report answers: the 200
// rollup with counts plus top findings per kind, the selector and
// top_per_kind 400s, the live 403, the 501 capability, the
// fixed-message 500, the retryable 503, and the gateway 504.
func divergenceReportOutcomes() []divergenceParityOutcome {
	return []divergenceParityOutcome{
		{"rollup 200", http.StatusOK, nil, `{"repo_id":"repo-1","counts":{"exact":1,"renamed":0,"drifted":0,"wrapper_bypass":0,"convention_outlier":0},"total":1,"top":{"exact":[{"finding_id":"exact:fp-1"}]},"top_per_kind":3,"truncated":{},"suppressions":{},"source_backend":"postgres_content_store"}`},
		{"validation 400 repo_id required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repo_id is required"}`},
		{"validation 400 top_per_kind range", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"top_per_kind must be <= 10"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"code divergence findings require a supported query profile"}`},
		{"fixed-message 500 report failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"divergence report read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, divergenceRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// TestDivergenceTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for all three
// terminated routes: status, headers, and body cross identically in
// both negotiated shapes, for every route and every driven body
// shape (all-families, repo-scoped, single-family, point lookup,
// target lookup, default rollup, top-capped rollup).
func TestDivergenceTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path     string
		bodies   []string
		outcomes []divergenceParityOutcome
	}{
		{divergenceFindingsPath, divergenceFindingsBodies, divergenceFindingsOutcomes()},
		{divergenceInvestigatePath, divergenceInvestigateBodies, divergenceInvestigateOutcomes()},
		{divergenceReportPath, divergenceReportBodies, divergenceReportOutcomes()},
	}
	for _, leg := range legs {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			for _, body := range leg.bodies {
				for _, outcome := range leg.outcomes {
					name := "POST " + leg.path + " " + outcome.name + " " + accept
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
						nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, divergenceAuthedPost(leg.path, body, accept))

						relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
						relayRec := httptest.NewRecorder()
						relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, divergenceAuthedPost(leg.path, body, accept))

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
