// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Code-divergence byte-parity matrix (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#125). The stub core answers every
// documented divergence outcome per route, so the lane proves parity
// on the full outcome set the arbiter bar names without inventing
// any handler behavior dest-side: the findings exact and drifted
// pages plus the scoped-empty zero page, the validation and selector
// 400s, the investigate point lookups with their unknown-fingerprint
// 404s, the report rollup with its top-bounded slice, plus
// grant-denied 403s, capability 501s, fixed-message 500s, the
// configuration-state 503, retryable 503s with Retry-After, and
// gateway 504s, in both negotiated shapes.

type codeDivergenceParityOutcome struct {
	name   string
	status int
	header http.Header
	body   string
}

// codeDivergenceRetryable503 is the shared retryable-core shape: the
// lane relays it with its Retry-After intact, never converting it to
// a local 503.
var codeDivergenceRetryable503 = http.Header{"Retry-After": []string{"5"}}

// codeDivergenceFindingsOutcomes pins the documented findings
// answers the matrix drives: 200 found and scoped-empty zero pages,
// the validation and selector 400s, the grant-denied 403, the 501
// capability, the fixed-message 500, the configuration-state and
// retryable 503s, and the gateway 504.
func codeDivergenceFindingsOutcomes() []codeDivergenceParityOutcome {
	return []codeDivergenceParityOutcome{
		{"found 200", http.StatusOK, nil, `{"repo_id":"repo-1","kind":"exact","limit":25,"offset":0,"findings":[{"finding_id":"exact:fp-1","kind":"parallel_implementation.exact","score":420,"members":[{"entity_id":"e1"},{"entity_id":"e2"}]}],"count":1,"truncated":false,"next_offset":null,"suppressions":{"generated_file":1},"source_backend":"postgres_content_store"}`},
		{"scoped-empty zero page 200", http.StatusOK, nil, `{"repo_id":"repo-1","kind":"","limit":25,"offset":0,"findings":[],"count":0,"truncated":false,"next_offset":null,"suppressions":{},"source_backend":"postgres_content_store"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 repo_id required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repo_id is required"}`},
		{"validation 400 bad kind", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"kind must be one of: \"\", \"exact\", \"renamed\", \"drifted\", \"wrapper_bypass\", \"convention_outlier\" (qualified \"parallel_implementation.*\" spellings accepted)"}`},
		{"validation 400 over limit", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"limit must be <= 100"}`},
		{"validation 400 over offset", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"offset must be <= 10000"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"code divergence findings require a supported query profile"}`},
		{"fixed-message 500 findings read failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"divergence findings read failed"}`},
		{"unavailable 503 no retry-after", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"divergence findings content index is unavailable"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeDivergenceRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// codeDivergenceInvestigateOutcomes pins the documented investigate
// answers: 200 addressed findings with bounded next steps, the
// validation and unknown-fingerprint 400/404s, the grant-denied 403,
// the 501 capability, the fixed-message 500, the configuration-state
// and retryable 503s, and the gateway 504.
func codeDivergenceInvestigateOutcomes() []codeDivergenceParityOutcome {
	return []codeDivergenceParityOutcome{
		{"found 200", http.StatusOK, nil, `{"repo_id":"repo-1","finding":{"finding_id":"exact:fp-1","kind":"parallel_implementation.exact","score":420,"members":[{"entity_id":"e1"},{"entity_id":"e2"}]},"next_steps":[{"tool":"find_function_call_chain","args":{"repo_id":"repo-1","start_entity_id":"e1"},"purpose":"trace callers"},{"tool":"get_file_lines","args":{"repo_id":"repo-1","relative_path":"a/maps.go","start_line":10,"end_line":60},"purpose":"read the copy"}],"truncated":false,"suppressions":{"generated_file":1},"source_backend":"postgres_content_store"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 repo_id required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repo_id is required"}`},
		{"validation 400 bad kind", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"kind must be one of: \"exact\", \"renamed\", \"drifted\", \"wrapper_bypass\", \"convention_outlier\" (qualified \"parallel_implementation.*\" spellings accepted)"}`},
		{"validation 400 fingerprint required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"fingerprint is required"}`},
		{"unknown fingerprint 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"divergence finding not found"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"code divergence findings require a supported query profile"}`},
		{"fixed-message 500 members read failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"divergence finding members read failed"}`},
		{"unavailable 503 no retry-after", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"divergence findings content index is unavailable"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeDivergenceRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// codeDivergenceReportOutcomes pins the documented report answers:
// 200 rollups with counts, totals, and top slices, the validation
// 400s, the grant-denied 403, the 501 capability, the fixed-message
// 500, the configuration-state and retryable 503s, and the gateway
// 504.
func codeDivergenceReportOutcomes() []codeDivergenceParityOutcome {
	return []codeDivergenceParityOutcome{
		{"rollup 200", http.StatusOK, nil, `{"repo_id":"repo-1","counts":{"parallel_implementation.exact":1,"parallel_implementation.renamed":0,"parallel_implementation.drifted":1,"parallel_implementation.wrapper_bypass":0,"parallel_implementation.convention_outlier":0},"total":2,"top":{"parallel_implementation.exact":[{"finding_id":"exact:fp-1","score":420}]},"top_per_kind":3,"truncated":{"parallel_implementation.exact":false},"suppressions":{"generated_file":1},"source_backend":"postgres_content_store"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 repo_id required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repo_id is required"}`},
		{"validation 400 top_per_kind bound", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"top_per_kind must be <= 10"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"code divergence findings require a supported query profile"}`},
		{"fixed-message 500 report read failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"divergence report read failed"}`},
		{"unavailable 503 no retry-after", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"divergence findings content index is unavailable"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeDivergenceRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// TestCodeDivergenceTerminatedMatchesRelayByteForByte pins byte
// parity between the terminated wave lane and the generic relay for
// all three terminated routes: status, headers, and body cross
// identically in both negotiated shapes, for every route and every
// driven body shape (kind-selected and paged findings pages,
// exact/drifted/qualified point lookups, default and top-bounded
// rollups).
func TestCodeDivergenceTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path     string
		bodies   []string
		outcomes []codeDivergenceParityOutcome
	}{
		{codeDivergenceFindingsPath, codeDivergenceFindingsBodies, codeDivergenceFindingsOutcomes()},
		{codeDivergenceInvestigatePath, codeDivergenceInvestigateBodies, codeDivergenceInvestigateOutcomes()},
		{codeDivergenceReportPath, codeDivergenceReportBodies, codeDivergenceReportOutcomes()},
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
						nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, codeDivergenceAuthedPost(leg.path, body, accept))

						relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
						relayRec := httptest.NewRecorder()
						relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, codeDivergenceAuthedPost(leg.path, body, accept))

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
