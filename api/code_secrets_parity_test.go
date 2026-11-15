// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Code-secrets byte-parity matrix (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#98, W6c). The stub core answers every documented
// W6c outcome for the route, so the lane proves parity on the full
// outcome set the arbiter bar names without inventing any handler
// behavior dest-side: the side-table and legacy-scan 200s with their
// source labels and redaction, the blocked empty-grant page — plus
// validation/malformed 400s, the unsupported-kind 400, selector 400s,
// live 403s, capability 501s, the fixed-message 500, the
// configuration-state 503, retryable 503s with Retry-After, and
// gateway 504s, in both negotiated shapes. (The family-403 deny branch
// is unreachable pre-D8 and stays out of this matrix;
// TestCodeSecretsDenyBranchShapePinned pins its shape.)

type codeSecretsParityOutcome struct {
	name   string
	status int
	header http.Header
	body   string
}

// codeSecretsRetryable503 is the shared retryable-core shape: the
// lane relays it with its Retry-After intact, never converting it to
// a local 503.
var codeSecretsRetryable503 = http.Header{"Retry-After": []string{"5"}}

// codeSecretsOutcomes pins the documented secrets/investigate answers
// the matrix drives: the side-table and legacy-scan 200s (the
// source-label read-path shaping plus the redaction marker), the
// blocked empty-grant page, the validation and unsupported-kind
// 400s, the live 403, the 501 capability, the fixed-message 500, the
// configuration-state and retryable 503s, and the gateway 504.
func codeSecretsOutcomes() []codeSecretsParityOutcome {
	return []codeSecretsParityOutcome{
		{"investigate 200 side table", http.StatusOK, nil, `{"scope":{"repo_id":"repo-1","language":""},"finding_kinds":["api_token"],"findings":[{"rank":1,"repo_id":"repo-1","relative_path":"src/wire/handler.go","finding_kind":"api_token","line":"api_token = [REDACTED]"}],"recommended_next_calls":[],"count":1,"limit":25,"offset":0,"truncated":false,"source_backend":"postgres_content_store","coverage":{"query_shape":"content_secret_investigation","returned_count":1,"suppressed_count":0,"include_suppressed":false,"limit":25,"offset":0,"truncated":false,"redaction":"secret_values_replaced_with_redacted_marker","empty":false,"searched_all_kinds":false,"requires_repo_scope":false,"read_path":"side_table"}}`},
		{"legacy-scan 200 read path labeled", http.StatusOK, nil, `{"scope":{"repo_id":"repo-1","language":""},"finding_kinds":[],"findings":[{"rank":1,"repo_id":"repo-1","relative_path":"src/legacy/auth.py","finding_kind":"password_literal","line":"password = [REDACTED]"}],"recommended_next_calls":[],"count":1,"limit":25,"offset":0,"truncated":false,"source_backend":"postgres_content_store","coverage":{"query_shape":"content_secret_investigation","returned_count":1,"suppressed_count":0,"include_suppressed":false,"limit":25,"offset":0,"truncated":false,"redaction":"secret_values_replaced_with_redacted_marker","empty":false,"searched_all_kinds":true,"requires_repo_scope":false,"read_path":"legacy_scan","limitations":["the secret-line index is not published ready (a bulk load is in progress or its finalizer has not completed); this read scanned content_files and is slower, with identical findings"]}}`},
		{"grantless blocked 200 empty redacted page", http.StatusOK, nil, `{"scope":{"repo_id":"repo-1","language":""},"finding_kinds":["api_token"],"findings":[],"recommended_next_calls":[{"tool":"investigate_code_topic","reason":"broaden the security investigation when no redacted secret candidates were found"}],"count":0,"limit":25,"offset":0,"truncated":false,"source_backend":"postgres_content_store","coverage":{"query_shape":"content_secret_investigation","returned_count":0,"suppressed_count":0,"include_suppressed":false,"limit":25,"offset":0,"truncated":false,"redaction":"secret_values_replaced_with_redacted_marker","empty":true,"searched_all_kinds":false,"requires_repo_scope":false,"read_path":"side_table"}}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 offset negative", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"offset must be >= 0"}`},
		{"validation 400 offset too large", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"offset must be <= 10000"}`},
		{"validation 400 unsupported kind", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unsupported finding_kind \"nope\""}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"hardcoded secret investigation requires a supported query profile"}`},
		{"fixed-message 500 investigate failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"hardcoded secret investigation read failed"}`},
		{"unavailable 503 backend down", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"hardcoded secret investigation backend is unavailable"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeSecretsRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// TestCodeSecretsTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for the
// terminated route: status, headers, and body cross identically in
// both negotiated shapes, for the route and every driven body shape
// (kind-scoped and unscoped investigations).
func TestCodeSecretsTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		for _, body := range codeSecretsBodies {
			for _, outcome := range codeSecretsOutcomes() {
				name := "POST " + codeSecretsInvestigatePath + " " + outcome.name + " " + accept
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
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, codeSecretsAuthedPost(codeSecretsInvestigatePath, body, accept))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, codeSecretsAuthedPost(codeSecretsInvestigatePath, body, accept))

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

// TestCodeSecretsDecision2NotOwed pins the Decision 2 split for this
// lane: there is no tier-C mutation on the W6c secret surface, so no
// suppression-state filtering parity is owed. The wave entry is a
// read-only POST behind the W6c secret pin; the wave carries no
// mutation route, and the parity corpus above carries no mutation
// outcome. If a mutation ever joins this family, this test must fail
// and the lane must add the Decision 2 proof its contract names.
func TestCodeSecretsDecision2NotOwed(t *testing.T) {
	t.Parallel()

	for _, entry := range codeSecretsWave {
		if entry.Method != "POST" {
			t.Errorf("wave entry %s %s: method = %q, want POST (read-only; a mutation would owe Decision 2)", entry.Method, entry.Path, entry.Method)
		}
		if entry.Path != codeSecretsInvestigatePath {
			t.Errorf("wave entry %s %s: unknown W6c path (a mutation here would owe Decision 2)", entry.Method, entry.Path)
		}
	}
	for _, outcome := range codeSecretsOutcomes() {
		switch outcome.status {
		case http.StatusOK, http.StatusBadRequest, http.StatusForbidden, http.StatusNotImplemented, http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		default:
			t.Errorf("outcome %q: status = %d, want a read-outcome status (no mutation outcome in this lane)", outcome.name, outcome.status)
		}
	}
}
