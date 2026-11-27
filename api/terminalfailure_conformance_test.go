// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Collector-failure acknowledgement tier C-a native-dispatch
// conformance (Refs eshu-hq/eshu#9669, part of #6707). TDD record:
// this suite proves the native acknowledge route relays core bytes
// exactly like the legacy generic proxy it replaces, so a RED here is
// a real divergence finding (outcome, Accept, plus both bodies), not a
// red-test artifact: the lane already terminates write-through, so any
// mismatch means the lanes disagree on the wire. Canonical route only:
// no trailing-slash twin.

// terminalFailureParityOutcomes is the outcome matrix the native-vs-relay
// parity proof drives: the 12 dispatch rows from
// TestTerminalFailureNative_WriteThroughRelaysCoreBytes plus the
// 503-retryable present-value row. The dispatch pin proves the core 503
// relays with its Retry-After absent (dest never invents the retry
// signal); the added row proves a present Retry-After relays too (dest
// never strips it). Every row below must show exactly one upstream hit
// on each lane.
var terminalFailureParityOutcomes = []struct {
	name   string
	status int
	header http.Header
	body   string
}{
	{"acknowledged", http.StatusOK, nil, `{"status":"acknowledged","idempotency_key":"k1","acknowledgement":{"collector_kind":"security_alert","instance_id":"i1","scope_id":"s1","work_item_id":"w1","failure_class":"fetch_error","failure_operation":"fetch","failed_at":"2026-11-05T00:00:00Z","acknowledged_at":"2026-11-05T00:01:00Z"}}`},
	{"duplicate", http.StatusOK, nil, `{"status":"duplicate","idempotency_key":"k1","acknowledgement":{"collector_kind":"security_alert","instance_id":"i1","scope_id":"s1","work_item_id":"w1","failure_class":"fetch_error","failure_operation":"fetch","failed_at":"2026-11-05T00:00:00Z","acknowledged_at":"2026-11-05T00:01:00Z"}}`},
	{"missing reason", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"reason is required and must say why the failure is expected"}`},
	{"missing key", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"idempotency_key is required to make acknowledgement safe under retries"}`},
	{"bad target", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"collector_kind, instance_id, and scope_id are required, and reason is at most 512 characters"}`},
	{"core 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"acknowledging a collector terminal failure requires an admin (all-scopes) token"}`},
	{"404 unclaimed", http.StatusNotFound, nil, `{"error":"Not Found","detail":"collector instance is not registered or not active for that collector_kind"}`},
	{"404 claimed", http.StatusNotFound, nil, `{"error":"Not Found","detail":"collector instance is not registered or not active for that collector_kind","key_consumed":true,"key_detail":"nothing was acknowledged, the idempotency key is now consumed, and a retry needs a new idempotency_key"}`},
	{"409 different target", http.StatusConflict, nil, `{"error":"Conflict","detail":"idempotency_key was already used with a different collector_kind, instance_id, or scope_id"}`},
	{"409 in progress", http.StatusConflict, nil, `{"error":"Conflict","detail":"an acknowledgement for this idempotency_key is already in progress"}`},
	{"422 refused", http.StatusUnprocessableEntity, nil, `{"status":"refused","reason":"no unresolved terminal failure inside the window on that instance and scope","detail":"nothing was acknowledged and the idempotency key was not consumed"}`},
	{"core 503 absent retry-after", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"collector terminal failure acknowledgement is not configured"}`},
	{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"collector terminal failure acknowledgement is not configured"}`},
}

// TestTerminalFailureNative_MatchesRelayByteForByte pins that the tier
// C-a clone relays core bytes exactly like the legacy generic proxy it
// replaces: same status, same headers (modulo per-hop framing), same
// body — across the pinned outcome matrix in both negotiated shapes.
// The legacy lane is replayed by mounting the generic dispatch for the
// same path in a test mux, so the comparison is native-vs-relay on the
// same binary and stub core, not a copy of the implementation. Both
// lanes must show exactly one upstream hit: dest never retries a
// forwarded mutation.
func TestTerminalFailureNative_MatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	const route = "POST /api/v0/admin/collector-failures/acknowledge"
	method, path, _ := strings.Cut(route, " ")
	if path != strings.TrimPrefix(string(terminalFailureRoute), "POST ") {
		t.Fatalf("parity route %q drifts from terminalFailureRoute %q", route, terminalFailureRoute)
	}
	for _, outcome := range terminalFailureParityOutcomes {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)

			nativeSrv := NewServer(upstream.URL, skeletonTestKey)
			nativeHandler := nativeSrv.Handler(DefaultV0Sunset)
			nativeReq := authedRequest(method, path)
			nativeReq.Header.Set("Accept", accept)
			nativeReq.Body = io.NopCloser(strings.NewReader(ackPayload))
			nativeRec := httptest.NewRecorder()
			nativeHandler.ServeHTTP(nativeRec, nativeReq)
			nativeResult := nativeRec.Result()
			nativeBody, _ := io.ReadAll(nativeResult.Body)
			upstream.Close()

			if core.hits != 1 {
				t.Errorf("%s Accept %q: native upstream hits = %d, want exactly 1 (no retry on write-through)",
					outcome.name, accept, core.hits)
			}

			relayCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
			relayMux := http.NewServeMux()
			relayMux.Handle("/", relayCore.handler())
			relayUpstream := httptest.NewServer(relayMux)
			defer relayUpstream.Close()

			relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
			legacy := relaySrv.instrumented(viaRelay, RequireAuth(relaySrv.apiKey, http.HandlerFunc(relaySrv.dispatch)))
			relayReq := authedRequest(method, path)
			relayReq.Header.Set("Accept", accept)
			relayReq.Body = io.NopCloser(strings.NewReader(ackPayload))
			relayRec := httptest.NewRecorder()
			legacy.ServeHTTP(relayRec, relayReq)
			relayResult := relayRec.Result()
			relayBody, _ := io.ReadAll(relayResult.Body)

			if relayCore.hits != 1 {
				t.Errorf("%s Accept %q: relay upstream hits = %d, want exactly 1 (no retry on the legacy lane either)",
					outcome.name, accept, relayCore.hits)
			}
			if nativeRec.Code != relayRec.Code {
				t.Errorf("%s Accept %q: native status = %d, relay status = %d, want equal",
					outcome.name, accept, nativeRec.Code, relayRec.Code)
			}
			if string(nativeBody) != string(relayBody) {
				t.Errorf("%s Accept %q: native body = %q, relay body = %q, want byte-identical",
					outcome.name, accept, nativeBody, relayBody)
			}
			for _, key := range []string{"Retry-After", "Content-Type"} {
				if got, want := nativeResult.Header.Get(key), relayResult.Header.Get(key); got != want {
					t.Errorf("%s Accept %q: native %s = %q, relay %s = %q, want equal",
						outcome.name, accept, key, got, key, want)
				}
			}
		}
	}
}
