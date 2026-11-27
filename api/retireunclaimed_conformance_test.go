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

// Workflow-retire-unclaimed native-dispatch conformance (Refs
// eshu-hq/eshu#9669, part of #6707). TDD record: this suite proves the
// native retire-unclaimed route relays core bytes exactly like the
// legacy generic proxy it replaces, so a RED here is a real divergence
// finding (outcome, Accept, plus both bodies), not a red-test
// artifact: the lane already terminates write-through, so any mismatch
// means the lanes disagree on the wire. Canonical route only: no
// trailing-slash twin.

// retireUnclaimedParityOutcomes is the outcome matrix the
// native-vs-relay parity proof drives: the 8 dispatch rows from
// TestRetireUnclaimedNative_WriteThroughRelaysCoreBytes plus the
// 503-permanent row (no Retry-After: dest never invents the retry
// signal, and a permanent failure must stay permanent). Every row below
// must show exactly one upstream hit on each lane.
var retireUnclaimedParityOutcomes = []struct {
	name   string
	status int
	header http.Header
	body   string
}{
	{"retired", http.StatusOK, nil, `{"count":2,"items":[{"work_item_id":"w-1","scope_id":"s-1"},{"work_item_id":"w-2","scope_id":"s-2"}]}`},
	{"empty noop", http.StatusOK, nil, `{"count":0,"items":[]}`},
	{"blank kind", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"collector_kind and collector_instance_id are required and must not be empty"}`},
	{"unknown instance", http.StatusNotFound, nil, `{"error":"Not Found","detail":"retire unclaimed workflow work items: unknown collector instance: sbom_attestation/never-existed"}`},
	{"still configured", http.StatusConflict, nil, `{"error":"Conflict","detail":"retire unclaimed workflow work items: collector instance still configured: sbom_attestation/sbom-live"}`},
	{"core 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"workflow retirement requires an admin (all-scopes) token"}`},
	{"store fault", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"retire unclaimed workflow work items request failed"}`},
	{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"workflow retirement request failed"}`},
	{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the admin store is not configured"}`},
}

// TestRetireUnclaimedNative_MatchesRelayByteForByte pins that the
// retire-unclaimed clone relays core bytes exactly like the legacy
// generic proxy it replaces: same status, same headers (modulo per-hop
// framing), same body — across the pinned outcome matrix in both
// negotiated shapes. The legacy lane is replayed by mounting the
// generic dispatch for the same path in a test mux, so the comparison
// is native-vs-relay on the same binary and stub core, not a copy of
// the implementation. Both lanes must show exactly one upstream hit:
// dest never retries a forwarded mutation.
func TestRetireUnclaimedNative_MatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	const route = "POST /api/v0/admin/workflow-retire-unclaimed"
	method, path, _ := strings.Cut(route, " ")
	if path != strings.TrimPrefix(string(retireUnclaimedRoute), "POST ") {
		t.Fatalf("parity route %q drifts from retireUnclaimedRoute %q", route, retireUnclaimedRoute)
	}
	for _, outcome := range retireUnclaimedParityOutcomes {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)

			nativeSrv := NewServer(upstream.URL, skeletonTestKey)
			nativeHandler := nativeSrv.Handler(DefaultV0Sunset)
			nativeReq := authedRequest(method, path)
			nativeReq.Header.Set("Accept", accept)
			nativeReq.Body = io.NopCloser(strings.NewReader(retireUnclaimedPayload))
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
			relayReq.Body = io.NopCloser(strings.NewReader(retireUnclaimedPayload))
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
