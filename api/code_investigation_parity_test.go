// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Code-investigation byte-parity matrix (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#98, W6c). The stub core answers every
// documented W6c outcome per route, so the lane proves parity on the
// full outcome set the arbiter bar names without inventing any handler
// behavior dest-side: the inventory entity and function-count-by-file
// 200s with the allocated empty-grant blocked page, the topic ranked
// page with its deadline-partial shape — plus validation/malformed
// 400s, selector 400s, live 403s, capability 501s, fixed-message
// 500s, configuration-state 503s, retryable 503s with Retry-After,
// and gateway 504s, in both negotiated shapes. (The family-403 deny
// branch is unreachable pre-D8 and stays out of this matrix;
// TestCodeInvestigationDenyBranchShapePinned pins its shape.)

type codeInvestigationParityOutcome struct {
	name   string
	status int
	header http.Header
	body   string
}

// codeInvestigationRetryable503 is the shared retryable-core shape:
// the lane relays it with its Retry-After intact, never converting it
// to a local 503.
var codeInvestigationRetryable503 = http.Header{"Retry-After": []string{"5"}}

// codeInvestigationInventoryOutcomes pins the documented
// structure/inventory answers the matrix drives: the entity and
// function-count-by-file 200s, the grantless blocked 200 (allocated
// empty results, indistinguishable from an empty index), the
// validation and selector 400s, the live 403, the 501 capability,
// the fixed-message 500, the configuration-state and retryable 503s,
// and the gateway 504.
func codeInvestigationInventoryOutcomes() []codeInvestigationParityOutcome {
	return []codeInvestigationParityOutcome{
		{"entity 200", http.StatusOK, nil, `{"repo_id":"repo-1","language":"go","inventory_kind":"entity","entity_kind":"function","file_path":"","symbol":"","decorator":"","method_name":"","class_name":"","limit":25,"offset":0,"results":[{"entity_id":"e1","name":"handleWire","entity_name":"handleWire","entity_type":"Function","file_path":"src/wire/handler.go","relative_path":"src/wire/handler.go","repo_id":"repo-1","language":"go"}],"count":1,"truncated":false,"next_offset":null,"source_backend":"postgres_content_store"}`},
		{"file-count 200", http.StatusOK, nil, `{"repo_id":"repo-1","language":"","inventory_kind":"function_count_by_file","entity_kind":"","file_path":"","symbol":"","decorator":"","method_name":"","class_name":"","limit":25,"offset":0,"results":[{"file_path":"src/wire/handler.go","function_count":3}],"count":1,"truncated":false,"next_offset":null,"source_backend":"postgres_content_store"}`},
		{"grantless blocked 200 allocated empty", http.StatusOK, nil, `{"repo_id":"repo-1","language":"","inventory_kind":"entity","entity_kind":"function","file_path":"","symbol":"","decorator":"","method_name":"","class_name":"","limit":25,"offset":0,"results":[],"count":0,"truncated":false,"next_offset":null,"source_backend":"postgres_content_store"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 limit too large", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"limit must be <= 200"}`},
		{"validation 400 bad kind", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"inventory_kind must be one of: entity, top_level, dataclass, documented, documented_function, decorated, class_with_method, super_call, function_count_by_file"}`},
		{"validation 400 method_name required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"method_name is required for class_with_method inventory"}`},
		{"validation 400 scope required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"one of repo_id, file_path, language, entity_kind, or symbol is required"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"structural code inventory requires a supported query profile"}`},
		{"fixed-message 500 inventory failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"structural inventory read failed"}`},
		{"unavailable 503 index not ready", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"structural inventory content index is unavailable"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeInvestigationRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// codeInvestigationTopicsOutcomes pins the documented
// topics/investigate answers the matrix drives: the ranked 200, the
// deadline-partial 200 (bounded partial rows alongside the marker,
// `deadline_truncated` set), the grantless empty page, the validation
// and selector 400s, the live 403, the 501 capability, the
// fixed-message 500, the configuration-state and retryable 503s, and
// the gateway 504.
func codeInvestigationTopicsOutcomes() []codeInvestigationParityOutcome {
	return []codeInvestigationParityOutcome{
		{"investigate 200", http.StatusOK, nil, `{"topic":"wire retry","intent":"","scope":{"repo_id":"repo-1"},"searched_terms":["wire","retry"],"matched_files":[{"repo_id":"repo-1","relative_path":"src/wire/handler.go"}],"matched_symbols":[{"entity_id":"e1","name":"handleWire"}],"evidence_groups":[{"rank":1,"source_kind":"entity","repo_id":"repo-1","relative_path":"src/wire/handler.go","language":"go"}],"call_graph_handles":[{"entity_id":"e1"}],"recommended_next_calls":[],"count":1,"limit":25,"offset":0,"truncated":false,"candidate_pool_truncated":false,"deadline_truncated":false,"source_backend":"postgres_content_store","coverage":{"query_shape":"content_topic_investigation","searched_terms":["wire","retry"],"searched_term_count":2,"returned_count":1,"limit":25,"offset":0,"truncated":false,"candidate_pool_truncated":false,"deadline_truncated":false,"empty":false}}`},
		{"deadline-partial 200 bounded rows with marker", http.StatusOK, nil, `{"topic":"wire retry","intent":"","scope":{"repo_id":"repo-1"},"searched_terms":["wire"],"matched_files":[{"repo_id":"repo-1","relative_path":"src/wire/handler.go"}],"matched_symbols":[],"evidence_groups":[{"rank":1,"source_kind":"entity","repo_id":"repo-1","relative_path":"src/wire/handler.go","language":"go"}],"call_graph_handles":[],"recommended_next_calls":[],"count":1,"limit":25,"offset":0,"truncated":false,"candidate_pool_truncated":false,"deadline_truncated":true,"source_backend":"postgres_content_store","coverage":{"query_shape":"content_topic_investigation","searched_terms":["wire"],"searched_term_count":1,"returned_count":1,"limit":25,"offset":0,"truncated":false,"candidate_pool_truncated":false,"deadline_truncated":true,"empty":false}}`},
		{"grantless 200 empty page", http.StatusOK, nil, `{"topic":"wire retry","intent":"","scope":{"repo_id":"repo-1"},"searched_terms":["wire","retry"],"matched_files":[],"matched_symbols":[],"evidence_groups":[],"call_graph_handles":[],"recommended_next_calls":[],"count":0,"limit":25,"offset":0,"truncated":false,"candidate_pool_truncated":false,"deadline_truncated":false,"source_backend":"postgres_content_store","coverage":{"query_shape":"content_topic_investigation","searched_terms":["wire","retry"],"searched_term_count":2,"returned_count":0,"limit":25,"offset":0,"truncated":false,"candidate_pool_truncated":false,"deadline_truncated":false,"empty":true}}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 topic required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"topic is required"}`},
		{"validation 400 offset too large", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"offset must be <= 10000"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"code topic investigation requires a supported query profile"}`},
		{"fixed-message 500 topic failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"code topic investigation read failed"}`},
		{"unavailable 503 backend down", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"code topic investigation backend is unavailable"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeInvestigationRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// TestCodeInvestigationTerminatedMatchesRelayByteForByte pins byte
// parity between the terminated wave lane and the generic relay for
// both terminated routes: status, headers, and body cross identically
// in both negotiated shapes, for every route and every driven body
// shape (entity and file-count inventories, plain and intent-scoped
// topics).
func TestCodeInvestigationTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path     string
		bodies   []string
		outcomes []codeInvestigationParityOutcome
	}{
		{codeInvestigationInventoryPath, codeInvestigationInventoryBodies, codeInvestigationInventoryOutcomes()},
		{codeInvestigationTopicsPath, codeInvestigationTopicsBodies, codeInvestigationTopicsOutcomes()},
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
						nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, codeInvestigationAuthedPost(leg.path, body, accept))

						relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
						relayRec := httptest.NewRecorder()
						relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, codeInvestigationAuthedPost(leg.path, body, accept))

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

// TestCodeInvestigationDecision2NotOwed pins the Decision 2 split for
// this lane: there is no tier-C mutation on the W6c investigation
// surface, so no suppression-state filtering parity is owed. Both
// wave entries are read-only POSTs behind the W6c investigation pin;
// the wave carries no mutation route, and the parity corpus above
// carries no mutation outcome. If a mutation ever joins this family,
// this test must fail and the lane must add the Decision 2 proof its
// contract names.
func TestCodeInvestigationDecision2NotOwed(t *testing.T) {
	t.Parallel()

	for _, entry := range codeInvestigationWave {
		if entry.Method != "POST" {
			t.Errorf("wave entry %s %s: method = %q, want POST (read-only; a mutation would owe Decision 2)", entry.Method, entry.Path, entry.Method)
		}
		switch entry.Path {
		case codeInvestigationInventoryPath, codeInvestigationTopicsPath:
		default:
			t.Errorf("wave entry %s %s: unknown W6c path (a mutation here would owe Decision 2)", entry.Method, entry.Path)
		}
	}
	for _, outcome := range append(codeInvestigationInventoryOutcomes(), codeInvestigationTopicsOutcomes()...) {
		switch outcome.status {
		case http.StatusOK, http.StatusBadRequest, http.StatusForbidden, http.StatusNotImplemented, http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		default:
			t.Errorf("outcome %q: status = %d, want a read-outcome status (no mutation outcome in this lane)", outcome.name, outcome.status)
		}
	}
}
