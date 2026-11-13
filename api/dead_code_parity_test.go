// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Dead-code byte-parity matrix (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#124). The stub core answers every documented
// dead-code outcome per route, so the lane proves parity on the full
// outcome set the arbiter bar names without inventing any handler
// behavior dest-side: the scan candidates and truncated-scan shapes,
// the cross-repo live_by_consumer and unknown_needs_evidence shapes
// with the handles/full evidence detail, and the investigate packet
// with its coverage, maturity, and bucket shapes — plus
// validation/malformed 400s, ungranted-selector 400s, live 403s,
// capability 501s, fixed-message 500s, retryable 503s with Retry-After,
// and gateway 504s, in both negotiated shapes.

type deadCodeParityOutcome struct {
	name   string
	status int
	header http.Header
	body   string
}

// deadCodeRetryable503 is the shared retryable-core shape: the lane
// relays it with its Retry-After intact, never converting it to a
// local 503.
var deadCodeRetryable503 = http.Header{"Retry-After": []string{"5"}}

// deadCodeScanOutcomes pins the documented scan answers the matrix
// drives: 200 candidates and scoped-empty zero pages (a blocked grant
// answers the same 200 empty page), the truncated-scan shape, the
// selector and validation 400s, the live 403, the 501 capability, the
// fixed-message 500, the retryable 503, and the gateway 504.
func deadCodeScanOutcomes() []deadCodeParityOutcome {
	return []deadCodeParityOutcome{
		{"candidates 200", http.StatusOK, nil, `{"candidate_kind":"Function","repo_id":"repo-1","language":"go","limit":100,"truncated":false,"candidates":[{"id":"ent-1","name":"oldHelper","kind":"Function"}]}`},
		{"scoped-empty zero page 200", http.StatusOK, nil, `{"candidate_kind":"Function","repo_id":"repo-1","language":"go","limit":100,"truncated":false,"candidates":[]}`},
		{"truncated 200 scan cap left behind", http.StatusOK, nil, `{"candidate_kind":"Function","language":"go","limit":100,"truncated":true,"candidate_scan_truncated":true,"candidate_scan_limit":5000,"candidate_scan_pages":3,"candidates":[]}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unexpected end of JSON input"}`},
		{"validation 400 candidate_kind unsupported", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unsupported candidate_kind: Module"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"dead-code scan requires authoritative graph mode"}`},
		{"fixed-message 500 scan failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"dead-code candidate scan failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, deadCodeRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// deadCodeCrossRepoOutcomes pins the documented cross-repo answers:
// 200 live_by_consumer and unknown_needs_evidence shapes (full and
// handles detail), the selector and evidence_detail 400s, the
// ungranted 400, the live 403, the fixed-message 500, the retryable
// 503, and the gateway 504.
func deadCodeCrossRepoOutcomes() []deadCodeParityOutcome {
	return []deadCodeParityOutcome{
		{"live_by_consumer 200", http.StatusOK, nil, `{"repo_id":"repo-1","query_shape":"bounded_cross_repo_dead_code","rows":[{"entity_id":"ent-1","classification":"live_by_consumer","evidence":[{"consumer_repo_id":"repo-2","relationship_type":"CALLS"}]}]}`},
		{"unknown_needs_evidence 200", http.StatusOK, nil, `{"repo_id":"repo-1","query_shape":"bounded_cross_repo_dead_code","rows":[{"entity_id":"ent-9","classification":"unknown_needs_evidence","truth":{"omissions":["consumer snapshot incomplete"]}}]}`},
		{"handles 200 reduced", http.StatusOK, nil, `{"repo_id":"repo-1","query_shape":"bounded_cross_repo_dead_code","rows":[{"entity_id":"ent-1","classification":"live_by_consumer","evidence_groups":[{"consumer_repo_id":"repo-2","item_count":3}]}]}`},
		{"validation 400 repo_id required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repo_id is required"}`},
		{"validation 400 evidence_detail invalid", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unknown evidence_detail: compact"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"fixed-message 500 evidence failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"cross-repo consumer evidence read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, deadCodeRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// deadCodeInvestigateOutcomes pins the documented investigate answers:
// 200 packet and empty shapes with coverage and maturity, the
// validation and ungranted selector 400s, the live 403, the 501
// capability, the fixed-message 500, the retryable 503, and the
// gateway 504.
func deadCodeInvestigateOutcomes() []deadCodeParityOutcome {
	return []deadCodeParityOutcome{
		{"packet 200", http.StatusOK, nil, `{"repo_id":"repo-1","language":"go","cleanup_ready":[],"ambiguous":[],"coverage":{},"language_maturity":{}}`},
		{"empty 200", http.StatusOK, nil, `{"language":"go","cleanup_ready":[],"ambiguous":[],"coverage":{}}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unexpected end of JSON input"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"dead-code investigation requires authoritative graph mode"}`},
		{"fixed-message 500 investigation failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"dead-code investigation read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, deadCodeRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// TestDeadCodeTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for all three
// terminated routes: status, headers, and body cross identically in
// both negotiated shapes, for every route and every driven body
// shape (corpus-wide, repo-scoped, candidate-kind, consumer-scoped,
// handles detail).
func TestDeadCodeTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path     string
		bodies   []string
		outcomes []deadCodeParityOutcome
	}{
		{deadCodeScanPath, deadCodeScanBodies, deadCodeScanOutcomes()},
		{deadCodeCrossRepoPath, deadCodeCrossRepoBodies, deadCodeCrossRepoOutcomes()},
		{deadCodeInvestigatePath, deadCodeInvestigateBodies, deadCodeInvestigateOutcomes()},
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
						nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, deadCodeAuthedPost(leg.path, body, accept))

						relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
						relayRec := httptest.NewRecorder()
						relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, deadCodeAuthedPost(leg.path, body, accept))

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
