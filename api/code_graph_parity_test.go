// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Code-graph byte-parity matrix (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#121). The stub core answers every documented G-a
// outcome per route, so the lane proves parity on the full outcome
// set the arbiter bar names without inventing any handler behavior
// dest-side: the call-chain found and scoped-empty zero pages, the
// cross_repo selector shapes (repo-scoped, entity-id, explicit
// endpoint pair, depth-bounded), the compare path enumeration with
// its truncation/visited budget, the metrics exact build with its
// 422 fail-closed bound, and the routes/callers trace with its
// 409-ambiguity rule — plus validation/malformed 400s, ungranted
// selector 400s, endpoint 404s, live 403s, capability 501s,
// fixed-message 500s, retryable 503s with Retry-After, and gateway
// 504s, in both negotiated shapes.

type codeGraphParityOutcome struct {
	name   string
	status int
	header http.Header
	body   string
}

// codeGraphRetryable503 is the shared retryable-core shape: the lane
// relays it with its Retry-After intact, never converting it to a
// local 503.
var codeGraphRetryable503 = http.Header{"Retry-After": []string{"5"}}

// codeGraphCallChainOutcomes pins the documented call-chain answers
// the matrix drives: 200 found and scoped-empty zero page (a blocked
// grant answers the same 200 empty chains), the selector and
// disambiguation 400s, the live 403, the 501 capability, the
// fixed-message 500, the retryable 503, and the gateway 504.
func codeGraphCallChainOutcomes() []codeGraphParityOutcome {
	return []codeGraphParityOutcome{
		{"found 200", http.StatusOK, nil, `{"start":"handleLogin","end":"writeSession","start_entity_id":"ent-1","end_entity_id":"ent-2","repo_id":"repo-1","cross_repo":false,"chains":[{"chain":[{"id":"ent-1","name":"handleLogin"},{"id":"ent-2","name":"writeSession"}],"depth":1}]}`},
		{"scoped-empty zero page 200", http.StatusOK, nil, `{"start":"handleLogin","end":"writeSession","repo_id":"repo-1","cross_repo":false,"chains":[]}`},
		{"validation 400 missing start", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"start or start_entity_id is required"}`},
		{"validation 400 selector disagreement", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"start_repo_id and end_repo_id require cross_repo"}`},
		{"validation 400 cross_repo missing endpoint selector", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"cross_repo call-chain traversal requires start_repo_id or repo_id"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"disambiguation 400 candidate pairs", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"call-chain endpoints matched too many candidate pairs in repository \"repo-1\" to probe safely (120 > 100); pass start_entity_id and end_entity_id to disambiguate"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"call-chain analysis requires authoritative graph mode"}`},
		{"fixed-message 500 graph read failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"call chain graph read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeGraphRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// codeGraphCompareOutcomes pins the documented compare answers:
// 200 enumeration, empty, and truncation-budget shapes (paths plus
// truncated, max_depth, max_paths, visited, source_backend), the
// single-repo 400s (repo_id required, cross_repo refused), the
// endpoint 404, the live 403, the 501 capability, the fixed-message
// 500, the retryable 503, and the gateway 504.
func codeGraphCompareOutcomes() []codeGraphParityOutcome {
	return []codeGraphParityOutcome{
		{"paths 200", http.StatusOK, nil, `{"start":"a","end":"b","repo_id":"repo-1","paths":[{"nodes":[{"id":"ent-1","name":"a"},{"id":"ent-2","name":"b"}],"depth":1,"confidence":0.9}],"truncated":false,"max_depth":4,"max_paths":5,"source_backend":"graph","visited":7}`},
		{"empty 200", http.StatusOK, nil, `{"start":"a","end":"b","repo_id":"repo-1","paths":[],"truncated":false,"max_depth":4,"max_paths":5,"source_backend":"graph","visited":3}`},
		{"truncated 200 budget left behind", http.StatusOK, nil, `{"start":"a","end":"b","repo_id":"repo-1","paths":[{"nodes":[{"id":"ent-1","name":"a"}],"depth":0,"confidence":1}],"truncated":true,"max_depth":4,"max_paths":5,"source_backend":"graph","visited":1000}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unexpected end of JSON input"}`},
		{"validation 400 repo_id required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repo_id is required"}`},
		{"validation 400 cross_repo refused", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"cross_repo compare is not supported; scope both entities to repo_id"}`},
		{"endpoint 404 not found", http.StatusNotFound, nil, `{"error":"Not Found","detail":"call chain endpoint not found: \"a\" to \"b\" in repository \"repo-1\""}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"code path comparison requires authoritative graph mode"}`},
		{"fixed-message 500 graph read failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"code path comparison graph read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeGraphRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// codeGraphMetricsOutcomes pins the documented metrics answers: 200
// exact rows and the empty page, the validation and ungranted
// selector 400s, the live 403, the 422 scope-too-broad fail-closed
// bound (no partial rows), the 501 capability, the fixed-message
// 500, the retryable 503, and the gateway 504.
func codeGraphMetricsOutcomes() []codeGraphParityOutcome {
	return []codeGraphParityOutcome{
		{"rows 200", http.StatusOK, nil, `{"metric_type":"hub_functions","functions":[{"id":"ent-1","name":"f","calls":12}],"count":1,"limit":25,"offset":0,"truncated":false,"next_offset":null,"source_backend":"graph","coverage":{}}`},
		{"empty 200", http.StatusOK, nil, `{"metric_type":"recursive_functions","functions":[],"count":0,"limit":10,"offset":5,"truncated":false,"next_offset":null,"source_backend":"graph","coverage":{}}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unexpected end of JSON input"}`},
		{"validation 400 repo_id required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repo_id is required"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"scope-too-broad 422 no partial rows", http.StatusUnprocessableEntity, nil, `{"error":"Unprocessable Entity","detail":"call graph metrics scope exceeds internal edge scan limit"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"call graph metrics require a supported query profile"}`},
		{"fixed-message 500 read failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"call graph metrics read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeGraphRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// codeGraphCallersOutcomes pins the documented routes/callers
// answers: 200 complete, partial, and unsupported traces (exact
// handler plus bounded callers/callees, or the no-HANDLES_ROUTE-edge
// report), the selector 400s, the out-of-grant and missing 404s,
// the 409 ambiguity rule, the live 403, the 501 capability, the
// fixed-message 500s, the backend-unavailable and retryable 503s,
// and the gateway 504.
func codeGraphCallersOutcomes() []codeGraphParityOutcome {
	return []codeGraphParityOutcome{
		{"complete 200", http.StatusOK, nil, `{"status":"complete","partial":false,"truncated":false,"unsupported":[],"truth_source":"HANDLES_ROUTE","route":{"path":"/api/v0/users"},"handler":{"id":"h-1"},"callers":[{"id":"c-1"}],"callees":[],"impact":{},"max_depth":2,"limit":25}`},
		{"partial 200 truncated", http.StatusOK, nil, `{"status":"partial","partial":true,"truncated":true,"unsupported":[],"truth_source":"HANDLES_ROUTE","route":{"path":"/api/v0/users"},"handler":{"id":"h-1"},"callers":[],"callees":[],"impact":{},"max_depth":2,"limit":25}`},
		{"unsupported 200 no handler edge", http.StatusOK, nil, `{"status":"unsupported","partial":true,"truncated":false,"unsupported":["no exact HANDLES_ROUTE edge exists for this endpoint selector"],"route":{"path":"/api/v0/users"},"handler":null,"callers":[],"callees":[],"impact":{},"max_depth":2,"limit":25,"truth_source":"HANDLES_ROUTE"}`},
		{"validation 400 path required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"path is required"}`},
		{"validation 400 scope required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repo_id, service_id, or service_name is required"}`},
		{"out-of-grant 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"route not found"}`},
		{"missing 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"route not found"}`},
		{"ambiguous 409 retry narrower", http.StatusConflict, nil, `{"error":"Conflict","detail":"route selector matched multiple endpoints or handlers"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"route-to-caller tracing requires authoritative graph mode"}`},
		{"fixed-message 500 relationship read failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"route-to-caller relationship read failed"}`},
		{"backend-unavailable 503 no retry-after", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"route-to-caller tracing requires a configured graph backend"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeGraphRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// TestCodeGraphTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for all four
// terminated routes: status, headers, and body cross identically in
// both negotiated shapes, for every route and every driven body
// shape (repo-scoped, entity-id, cross_repo endpoint selectors,
// single-repo compare caps, metric types with paging, repo and
// service caller scopes).
func TestCodeGraphTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path     string
		bodies   []string
		outcomes []codeGraphParityOutcome
	}{
		{codeGraphCallChainPath, codeGraphCallChainBodies, codeGraphCallChainOutcomes()},
		{codeGraphComparePath, codeGraphCompareBodies, codeGraphCompareOutcomes()},
		{codeGraphMetricsPath, codeGraphMetricsBodies, codeGraphMetricsOutcomes()},
		{codeGraphRouteCallers, codeGraphCallersBodies, codeGraphCallersOutcomes()},
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
						nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, codeGraphAuthedPost(leg.path, body, accept))

						relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
						relayRec := httptest.NewRecorder()
						relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, codeGraphAuthedPost(leg.path, body, accept))

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
