// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Code-scan byte-parity matrix (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#98, W6b). The stub core answers every documented
// W6b outcome per route, so the lane proves parity on the full
// outcome set the arbiter bar names without inventing any handler
// behavior dest-side: the complexity by-name, by-ID, ranked-list,
// and stale-id fallback 200s (plus the by-ID enrichment
// hydration), the quality/inspect build with its thresholds and
// recommended calls, the imports/investigate page with its
// grantless empty page, the bundles page with its public-only grant
// model — plus validation/malformed 400s, selector 400s, live 403s,
// unknown-entity 404s, the complexity 409 ambiguity rule, the
// imports 422 scope-too-broad bound (no partial rows), capability
// 501s, fixed-message 500s, configuration-state 503s, retryable 503s
// with Retry-After, and gateway 504s, in both negotiated shapes.
// (The family-403 deny branch is unreachable pre-D8 and stays out of
// this matrix; TestCodeScanDenyBranchShapePinned pins its shape.)

type codeScanParityOutcome struct {
	name   string
	status int
	header http.Header
	body   string
}

// codeScanRetryable503 is the shared retryable-core shape: the lane
// relays it with its Retry-After intact, never converting it to a
// local 503.
var codeScanRetryable503 = http.Header{"Retry-After": []string{"5"}}

// codeScanComplexityOutcomes pins the documented complexity answers
// the matrix drives: the by-name, by-ID, ranked-list, and
// stale-id-fallback 200s, the grantless list and entity pages, the
// validation and selector 400s, the ambiguity 409, the unknown 404,
// the live 403, the fixed-message 500s, the retryable 503, and the
// gateway 504.
func codeScanComplexityOutcomes() []codeScanParityOutcome {
	return []codeScanParityOutcome{
		{"by-name 200", http.StatusOK, nil, `{"entity_id":"e1","name":"handleWire","labels":["Function"],"file_path":"src/wire/handler.go","repo_id":"repo-1","repo_name":"wire","language":"go","start_line":3,"end_line":9,"complexity":7,"outgoing_count":2,"incoming_count":1,"total_relationships":3}`},
		{"by-ID 200", http.StatusOK, nil, `{"entity_id":"e1","name":"handleWire","labels":["Function"],"file_path":"src/wire/handler.go","repo_id":"repo-1","repo_name":"wire","language":"go","start_line":3,"end_line":9,"complexity":7,"outgoing_count":2,"incoming_count":1,"total_relationships":3}`},
		{"stale-id fallback 200", http.StatusOK, nil, `{"entity_id":"e9","name":"handleWire","labels":["Function"],"file_path":"src/wire/other.go","repo_id":"repo-1","repo_name":"wire","language":"go","start_line":40,"end_line":48,"complexity":4,"outgoing_count":1,"incoming_count":0,"total_relationships":1}`},
		{"list 200", http.StatusOK, nil, `{"repo_id":"repo-1","results":[{"entity_id":"e1","name":"handleWire","labels":["Function"],"file_path":"src/wire/handler.go","repo_id":"repo-1","repo_name":"wire","language":"go","start_line":3,"end_line":9,"complexity":12}],"limit":5,"truncated":false,"result_key":"entity_id"}`},
		{"grantless list 200 empty page", http.StatusOK, nil, `{"repo_id":"repo-1","results":[],"limit":10,"truncated":false,"result_key":"entity_id"}`},
		{"grantless entity 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"entity not found"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"ambiguous 409 retry narrower", http.StatusConflict, nil, `{"status":"ambiguous","function_name":"handleWire","repo_id":"repo-1","candidates":[{"entity_id":"e1","name":"handleWire"},{"entity_id":"e2","name":"handleWire"}],"truncated":false,"error":"Conflict","detail":"function_name \"handleWire\" matched multiple entities in repository \"repo-1\""}`},
		{"unknown 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"entity not found"}`},
		{"fixed-message 500 ranking failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"complexity ranking read failed"}`},
		{"fixed-message 500 lookup failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"complexity lookup read failed"}`},
		{"fixed-message 500 metadata failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"complexity content metadata read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeScanRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// codeScanQualityOutcomes pins the documented quality/inspect
// answers: the inspect 200 with thresholds and recommended calls,
// the grantless page, the validation and unsupported-check 400s,
// the live 403, the 501 capability, the fixed-message 500, the
// retryable 503, and the gateway 504.
func codeScanQualityOutcomes() []codeScanParityOutcome {
	return []codeScanParityOutcome{
		{"inspect 200", http.StatusOK, nil, `{"source":"graph","source_backend":"graph","check":"complexity","repo_id":"repo-1","language":"go","limit":10,"offset":0,"truncated":false,"result_key":"entity_id","thresholds":{"min_complexity":10,"min_lines":50,"min_arguments":5},"results":[{"entity_id":"e1","name":"handleWire","repo_id":"repo-1","file_path":"src/wire/handler.go","start_line":3,"end_line":9}],"recommended_next_calls":[{"tool":"get_file_lines","repo_id":"repo-1","relative_path":"src/wire/handler.go","start_line":3,"end_line":9}]}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 unsupported check", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unsupported code quality check"}`},
		{"validation 400 offset too large", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"offset exceeds maximum"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"code quality inspection requires authoritative graph mode"}`},
		{"fixed-message 500 inspect failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"code quality inspection read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeScanRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// codeScanImportsOutcomes pins the documented imports/investigate
// answers: the investigation 200, the grantless empty page (no
// backend read), the validation 400s, the scope-too-broad 422 (no
// partial rows), the live 403, the 501 capability, the
// fixed-message 500, the retryable 503, and the gateway 504.
func codeScanImportsOutcomes() []codeScanParityOutcome {
	return []codeScanParityOutcome{
		{"investigate 200", http.StatusOK, nil, `{"query_type":"imports_by_file","scope":{"repo_id":"repo-1","source_file":"src/wire/handler.go"},"imports":[{"source_file":"src/wire/handler.go","target_module":"store"}],"count":1,"limit":25,"offset":0,"truncated":false,"has_more":false,"next_offset":null,"source_backend":"graph","coverage":{"query_shape":"content_imports_investigation"}}`},
		{"grantless 200 empty page no read", http.StatusOK, nil, `{"query_type":"imports_by_file","scope":{"repo_id":""},"imports":[],"count":0,"limit":25,"offset":0,"truncated":false,"has_more":false,"next_offset":null,"source_backend":"no_backend_read"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 bad query_type", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"query_type must be one of: imports_by_file, importers, module_dependencies, package_imports, file_import_cycles, cross_module_calls"}`},
		{"validation 400 no scope", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"one of repo_id, source_file, target_file, source_module, target_module, or target_module_prefix is required"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"scope-too-broad 422 no partial rows", http.StatusUnprocessableEntity, nil, `{"error":"Unprocessable Entity","detail":"import dependency scope exceeds internal scan limit; narrow the repository, file, or module scope"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"import dependency investigation requires a supported query profile"}`},
		{"fixed-message 500 investigate failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"import dependency investigation read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeScanRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// codeScanBundlesOutcomes pins the documented bundles answers: the
// bundles 200, the scoped public-only 200, the grantless empty page
// (no graph call), the validation and scope 400s, the live 403, the
// fixed-message 500s, the retryable 503, and the gateway 504.
func codeScanBundlesOutcomes() []codeScanParityOutcome {
	return []codeScanParityOutcome{
		{"bundles 200", http.StatusOK, nil, `{"bundles":[{"package_id":"pkg-1","name":"wire","ecosystem":"go","registry":"proxy","namespace":"","purl":"pkg:golang/wire","version_count":3}],"count":1,"limit":50,"truncated":false}`},
		{"scoped public-only 200", http.StatusOK, nil, `{"bundles":[{"package_id":"pkg-1","name":"wire","ecosystem":"go","registry":"proxy","namespace":"","purl":"pkg:golang/wire","version_count":3}],"count":1,"limit":50,"truncated":false}`},
		{"grantless 200 empty page no read", http.StatusOK, nil, `{"bundles":[],"count":0,"limit":50,"truncated":false}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 scope required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"a non-empty query or ecosystem scope is required"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"fixed-message 500 search failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"registry bundle search read failed"}`},
		{"fixed-message 500 version count failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"registry bundle version count read failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeScanRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// TestCodeScanTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for all
// four terminated routes: status, headers, and body cross
// identically in both negotiated shapes, for every route and every
// driven body shape (by-name, by-ID, ranked list, checks, file and
// module scopes, query and ecosystem scopes).
func TestCodeScanTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path     string
		bodies   []string
		outcomes []codeScanParityOutcome
	}{
		{codeScanComplexityPath, codeScanComplexityBodies, codeScanComplexityOutcomes()},
		{codeScanQualityPath, codeScanQualityBodies, codeScanQualityOutcomes()},
		{codeScanImportsPath, codeScanImportsBodies, codeScanImportsOutcomes()},
		{codeScanBundlesPath, codeScanBundlesBodies, codeScanBundlesOutcomes()},
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
						nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, codeScanAuthedPost(leg.path, body, accept))

						relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
						relayRec := httptest.NewRecorder()
						relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, codeScanAuthedPost(leg.path, body, accept))

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

// TestCodeScanDecision2NotOwed pins the Decision 2 split for this
// lane: there is no tier-C mutation on the W6b surface, so no
// suppression-state filtering parity is owed. All four wave entries
// are read-only POSTs behind the W6b scan pin; the wave carries no
// mutation route, and the parity corpus above carries no mutation
// outcome. If a mutation ever joins this family, this test must fail
// and the lane must add the Decision 2 proof its contract names.
func TestCodeScanDecision2NotOwed(t *testing.T) {
	t.Parallel()

	for _, entry := range codeScanWave {
		if entry.Method != "POST" {
			t.Errorf("wave entry %s %s: method = %q, want POST (read-only; a mutation would owe Decision 2)", entry.Method, entry.Path, entry.Method)
		}
		switch entry.Path {
		case codeScanComplexityPath, codeScanQualityPath, codeScanImportsPath, codeScanBundlesPath:
		default:
			t.Errorf("wave entry %s %s: unknown W6b path (a mutation here would owe Decision 2)", entry.Method, entry.Path)
		}
	}
	for _, outcome := range append(append(append(codeScanComplexityOutcomes(), codeScanQualityOutcomes()...), codeScanImportsOutcomes()...), codeScanBundlesOutcomes()...) {
		switch outcome.status {
		case http.StatusOK, http.StatusBadRequest, http.StatusNotFound, http.StatusForbidden, http.StatusConflict, http.StatusNotImplemented, http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusUnprocessableEntity, http.StatusGatewayTimeout:
		default:
			t.Errorf("outcome %q: status = %d, want a read-outcome status (no mutation outcome in this lane)", outcome.name, outcome.status)
		}
	}
}
