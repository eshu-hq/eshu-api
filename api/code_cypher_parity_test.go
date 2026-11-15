// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Code-cypher byte-parity matrix (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#98, W6d). The stub core answers every documented
// W6d outcome per route, so the lane proves parity on the full
// outcome set the arbiter bar names without inventing any handler
// behavior dest-side: the cypher raw-row 200s and the visualize
// renderable-packet 200s (plus the truncation-limitation shape) —
// plus validation/malformed 400s, the mutation-keyword and
// LIMIT-bound 400s, the statement-rejection 400, the live 403s, the
// 501 capabilities, the fixed-message 500s, retryable 503s with
// Retry-After, and gateway 504s, in both negotiated shapes. The
// shared-key-only grant-class gate is ported by the wave gate and
// proven by TestCodeCypherWaveRelayExplicitForUnresolvableIdentities:
// a scoped or browser-session caller relays explicitly and core
// judges it there. (The family-403 deny branch is unreachable pre-D8
// and stays out of this matrix;
// TestCodeCypherDenyBranchShapePinned pins its shape.)

type codeCypherParityOutcome struct {
	name   string
	status int
	header http.Header
	body   string
}

// codeCypherRetryable503 is the shared retryable-core shape: the lane
// relays it with its Retry-After intact, never converting it to a
// local 503.
var codeCypherRetryable503 = http.Header{"Retry-After": []string{"5"}}

// codeCypherQueryOutcomes pins the documented cypher answers the
// matrix drives: the raw-row 200s, the validation, mutation-keyword,
// over-length, and LIMIT-bound 400s, the statement-rejection 400, the
// live 403, the 501 capability, the fixed-message 500, the retryable
// 503, and the gateway 504.
func codeCypherQueryOutcomes() []codeCypherParityOutcome {
	return []codeCypherParityOutcome{
		{"rows 200", http.StatusOK, nil, `{"results":[{"n":{"id":"e1","name":"handleWire"}}],"limit":100,"truncated":false}`},
		{"rows 200 limit-pinned truncated", http.StatusOK, nil, `{"results":[{"n":{"id":"e1"}},{"n":{"id":"e2"}}],"limit":2,"truncated":true}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 query required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"cypher_query is required"}`},
		{"validation 400 mutation keyword", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"query contains disallowed keyword \"DELETE\"; only read-only queries are permitted"}`},
		{"validation 400 over length", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"query exceeds maximum length of 4096 characters"}`},
		{"validation 400 limit exceeds requested", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"query LIMIT 50 exceeds requested limit 25"}`},
		{"statement-rejection 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"the graph rejected the Cypher statement: unsupported clause"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"read-only Cypher queries require a graph-backed authoritative profile"}`},
		{"fixed-message 500 query failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"read-only Cypher query failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeCypherRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// codeCypherVisualizeOutcomes pins the documented visualize answers
// the matrix drives: the renderable-packet 200, the row-window
// truncation 200 (with its limitation reason), the no-entities 200,
// the validation, mutation-keyword, and LIMIT-bound 400s, the
// statement-rejection 400, the live 403, the 501 capability, the
// fixed-message 500, the retryable 503, and the gateway 504.
func codeCypherVisualizeOutcomes() []codeCypherParityOutcome {
	return []codeCypherParityOutcome{
		{"packet 200", http.StatusOK, nil, `{"visualization_packet":{"nodes":[{"id":"e1","name":"handleWire"}],"edges":[{"source_id":"e1","target_id":"e2"}],"truncated":false,"limitations":[]},"limit":100,"truncated":false}`},
		{"packet 200 window truncated", http.StatusOK, nil, `{"visualization_packet":{"nodes":[{"id":"e1"}],"edges":[],"truncated":true,"limitations":["result row window was truncated to the row limit before projection; the subgraph is a bounded subset"]},"limit":1,"truncated":true}`},
		{"packet 200 no entities", http.StatusOK, nil, `{"visualization_packet":{"nodes":[],"edges":[],"truncated":false,"limitations":["query returned no graph nodes, relationships, or paths to visualize; RETURN whole nodes/relationships/paths (for example RETURN n, r, m) rather than scalar properties"]},"limit":100,"truncated":false}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 query required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"cypher_query is required"}`},
		{"validation 400 mutation keyword", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"query contains disallowed keyword \"CREATE\"; only read-only queries are permitted"}`},
		{"validation 400 limit exceeds requested", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"query LIMIT 50 exceeds requested limit 25"}`},
		{"statement-rejection 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"the graph rejected the Cypher statement: unsupported clause"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"graph query visualization requires a graph-backed authoritative profile"}`},
		{"fixed-message 500 visualize failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"visualization graph query failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeCypherRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// TestCodeCypherTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for both
// terminated routes: status, headers, and body cross identically in
// both negotiated shapes, for every route and every driven body shape
// (plain and limit-pinned queries).
func TestCodeCypherTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	legs := []struct {
		path     string
		bodies   []string
		outcomes []codeCypherParityOutcome
	}{
		{codeCypherQueryPath, codeCypherQueryBodies, codeCypherQueryOutcomes()},
		{codeCypherVisualizePath, codeCypherVisualizeBodies, codeCypherVisualizeOutcomes()},
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
						nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, codeCypherAuthedPost(leg.path, body, accept))

						relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
						relayRec := httptest.NewRecorder()
						relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, codeCypherAuthedPost(leg.path, body, accept))

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

// TestCodeCypherDecision2NotOwed pins the Decision 2 split for this
// lane: there is no tier-C mutation on the W6d surface, so no
// suppression-state filtering parity is owed. Both wave entries are
// read-only POSTs behind the W6d cypher pin; the wave carries no
// mutation route, and the parity corpus above carries no mutation
// outcome. If a mutation ever joins this family, this test must fail
// and the lane must add the Decision 2 proof its contract names.
func TestCodeCypherDecision2NotOwed(t *testing.T) {
	t.Parallel()

	for _, entry := range codeCypherWave {
		if entry.Method != "POST" {
			t.Errorf("wave entry %s %s: method = %q, want POST (read-only; a mutation would owe Decision 2)", entry.Method, entry.Path, entry.Method)
		}
		switch entry.Path {
		case codeCypherQueryPath, codeCypherVisualizePath:
		default:
			t.Errorf("wave entry %s %s: unknown W6d path (a mutation here would owe Decision 2)", entry.Method, entry.Path)
		}
	}
	for _, outcome := range append(codeCypherQueryOutcomes(), codeCypherVisualizeOutcomes()...) {
		switch outcome.status {
		case http.StatusOK, http.StatusBadRequest, http.StatusForbidden, http.StatusNotImplemented, http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		default:
			t.Errorf("outcome %q: status = %d, want a read-outcome status (no mutation outcome in this lane)", outcome.name, outcome.status)
		}
	}
}
