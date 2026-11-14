// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Relationships byte-parity matrix (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#122). The stub core answers every documented G-b
// outcome per route, so the lane proves parity on the full outcome
// set the arbiter bar names without inventing any handler behavior
// dest-side: the direct graph 200, the content-fallback 200 via the
// narrowed builder closure, the transitive CALLS 200, the
// scoped-grant 200s and denials, the story class_hierarchy and
// overrides 200s with their enrichment blocks, plus validation and
// selector 400s, grant-denied 403s, unknown-entity 404s, capability
// 501s, fixed-message 500s, configuration-state 503s, retryable 503s
// with Retry-After, and gateway 504s, in both negotiated shapes.

type relationshipParityOutcome struct {
	name   string
	status int
	header http.Header
	body   string
}

// relationshipRetryable503 is the shared retryable-core shape: the
// lane relays it with its Retry-After intact, never converting it to
// a local 503.
var relationshipRetryable503 = http.Header{"Retry-After": []string{"5"}}

// relationshipOutcomes pins the documented relationships answers the
// matrix drives: the direct graph 200, the content-fallback 200 (no
// graph row, builder closure answers), the transitive CALLS 200, the
// scoped-grant 200 (neighbours clipped to the grant before the row
// ceiling), the validation and selector 400s, the grant-denied 403,
// the unknown-entity 404, the 501 capability, the fixed-message 500s,
// the configuration-state and retryable 503s, and the gateway 504.
func relationshipOutcomes() []relationshipParityOutcome {
	return []relationshipParityOutcome{
		{"direct 200", http.StatusOK, nil, `{"entity_id":"e1","name":"handleWire","labels":["Function"],"file_path":"src/wire/handler.go","repo_id":"repo-1","repo_name":"wire","language":"go","start_line":3,"end_line":9,"outgoing":[{"target_id":"e2","type":"CALLS"}],"incoming":[],"outgoing_truncated":false,"incoming_truncated":false}`},
		{"fallback 200 content builder", http.StatusOK, nil, `{"entity_id":"e1","name":"handleWire","labels":["Function"],"file_path":"src/wire/handler.go","repo_id":"repo-1","outgoing":[{"target_id":"e2","type":"CALLS"}],"incoming":[],"source_backend":"postgres_content_store"}`},
		{"transitive 200 CALLS", http.StatusOK, nil, `{"entity_id":"e1","name":"root","outgoing":[{"target_id":"e3","type":"CALLS","depth":2}],"incoming":[],"max_depth":5}`},
		{"scoped 200 grant-clipped", http.StatusOK, nil, `{"entity_id":"e1","name":"handleWire","repo_id":"repo-1","outgoing":[{"target_id":"e2","type":"CALLS"}],"incoming":[]}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 entity required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"entity_id or name is required"}`},
		{"validation 400 bad direction", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"direction must be one of: incoming, outgoing"}`},
		{"validation 400 transitive only CALLS", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"transitive relationships are only supported for CALLS"}`},
		{"validation 400 transitive needs direction", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"direction is required for transitive CALLS relationships"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 404 scoped unknown", http.StatusNotFound, nil, `{"error":"Not Found","detail":"entity not found"}`},
		{"unknown 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"entity not found"}`},
		{"capability 501 transitive", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"transitive relationships require a supported query profile"}`},
		{"fixed-message 500 graph failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"relationship graph read failed"}`},
		{"fixed-message 500 fallback failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"relationship content fallback failed"}`},
		{"unavailable 503 no retry-after", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"content relationship builder not configured"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, relationshipRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// relationshipStoryOutcomes pins the documented story answers: the
// resolved 200, the class_hierarchy 200 with parents/children/methods
// enrichment, the overrides 200 with override_story, the repo-scoped
// overrides 200, the ambiguous-candidates 200, the scoped not_found
// 200 (grant-blocked target answers not_found on the content basis,
// not a 403), the validation and selector 400s, the 501 capability,
// the fixed-message 500s, the configuration-state and retryable 503s,
// and the gateway 504.
func relationshipStoryOutcomes() []relationshipParityOutcome {
	return []relationshipParityOutcome{
		{"resolved 200", http.StatusOK, nil, `{"target":"class-a","status":"resolved","relationships":[{"target_id":"base","type":"INHERITS"}],"source_backend":"postgres_content_store"}`},
		{"class_hierarchy 200", http.StatusOK, nil, `{"target":"class-a","status":"resolved","relationships":[{"target_id":"base","type":"INHERITS"}],"class_hierarchy":{"parents":[{"target_id":"base"}],"children":[{"source_id":"child"}],"methods":[{"method_id":"method-run"}],"depth_summary":{"max_parent_depth":2},"methods_truncated":true},"coverage":{"query_shape":"entity_anchor_class_hierarchy_story","max_depth":4},"source_backend":"postgres_content_store"}`},
		{"overrides 200", http.StatusOK, nil, `{"target":"class-a","status":"resolved","relationships":[{"target_id":"base","type":"OVERRIDES"}],"override_story":{"overrides":[{"source_id":"child"}]},"source_backend":"postgres_content_store"}`},
		{"repo-scoped overrides 200", http.StatusOK, nil, `{"status":"repo_scoped","repo_id":"repo-1","override_story":{"overrides":[{"source_id":"child"}]},"source_backend":"postgres_content_store"}`},
		{"ambiguous 200 candidates", http.StatusOK, nil, `{"status":"ambiguous","candidates":[{"entity_id":"class-base"},{"entity_id":"function-base"}]}`},
		{"scoped not_found 200", http.StatusOK, nil, `{"status":"not_found","target":"class-a","source_backend":"postgres_content_store"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 class target", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"class_hierarchy target must resolve to a class or inheritable entity"}`},
		{"validation 400 repo required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repo_id is required for repo-scoped overrides"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"capability 501 story", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"code relationship story requires a supported query profile"}`},
		{"fixed-message 500 story failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"relationship story read failed"}`},
		{"fixed-message 500 hierarchy failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"relationship story hierarchy read failed"}`},
		{"unavailable 503 no retry-after", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"symbol backend is unavailable"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, relationshipRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// TestRelationshipTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for both
// terminated routes: status, headers, and body cross identically in
// both negotiated shapes, for every route and every driven body shape
// (direct, name-fallback, transitive CALLS, scoped grant reads,
// class_hierarchy, overrides, repo-scoped overrides).
func TestRelationshipTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path     string
		bodies   []string
		outcomes []relationshipParityOutcome
	}{
		{relationshipPath, relationshipBodies, relationshipOutcomes()},
		{relationshipStoryPath, relationshipStoryBodies, relationshipStoryOutcomes()},
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
						nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, relationshipAuthedPost(leg.path, body, accept))

						relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
						relayRec := httptest.NewRecorder()
						relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, relationshipAuthedPost(leg.path, body, accept))

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

// TestRelationshipDecision2NotOwed pins the Decision 2 split for this
// lane: there is no tier-C mutation on the G-b surface, so no
// suppression-state filtering parity is owed. Both wave entries are
// read-only POSTs behind the G-b read pin; the wave carries no
// mutation route, and the parity corpus above carries no mutation
// outcome. If a mutation ever joins this family, this test must fail
// and the lane must add the Decision 2 proof its contract names.
func TestRelationshipDecision2NotOwed(t *testing.T) {
	t.Parallel()

	for _, entry := range relationshipWave {
		if entry.Method != "POST" {
			t.Errorf("wave entry %s %s: method = %q, want POST (read-only; a mutation would owe Decision 2)", entry.Method, entry.Path, entry.Method)
		}
		switch entry.Path {
		case relationshipPath, relationshipStoryPath:
		default:
			t.Errorf("wave entry %s %s: unknown G-b path (a mutation here would owe Decision 2)", entry.Method, entry.Path)
		}
	}
	for _, outcome := range append(relationshipOutcomes(), relationshipStoryOutcomes()...) {
		switch outcome.status {
		case http.StatusOK, http.StatusBadRequest, http.StatusNotFound, http.StatusForbidden, http.StatusNotImplemented, http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		default:
			t.Errorf("outcome %q: status = %d, want a read-outcome status (no mutation outcome in this lane)", outcome.name, outcome.status)
		}
	}
}
