// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// Recovery tier C-c2 native-dispatch conformance (Refs eshu-hq/eshu#9669,
// part of #6707). TDD record: this suite pins the frozen route set first
// and then proves the four native routes relay core bytes exactly like
// the legacy generic proxy they replace, so a RED here is a real
// divergence finding (route, outcome, Accept, plus both bodies), not a
// red-test artifact: the lane already terminates write-through, so any
// mismatch means the lanes disagree on the wire.

// TestRecoveryRoutesFrozen pins the exact admin-recovery surface the
// satellite must clone. A route added, removed, or renamed fails here
// until the pin contract, the spec fragment, and this mirror move
// together. Canonical routes only: no trailing-slash twins.
func TestRecoveryRoutesFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/admin/recover-generations",
		"POST /api/v0/admin/refinalize",
		"POST /api/v0/admin/reopen",
		"POST /api/v0/admin/replay",
	}
	got := append([]string(nil), recoveryRoutes...)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("recoveryRoutes holds %d entries %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("recoveryRoutes differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// recoveryPayloadByPath carries the per-route caller bodies the parity
// proof forwards: the representative wire shapes from the dispatch pin
// (api/recovery_dispatch_test.go:123-132), one per route.
var recoveryPayloadByPath = map[string]string{
	"/api/v0/admin/replay":              `{"scope_id":"s1","reason":"retry after the fix deployed","idempotency_key":"k1"}`,
	"/api/v0/admin/reopen":              `{"domain":"workload_materialization","scope_id":"s1","reason":"hand repair","idempotency_key":"k3"}`,
	"/api/v0/admin/refinalize":          `{"scope_ids":["s1"]}`,
	"/api/v0/admin/recover-generations": `{"scope_ids":["s1"],"reason":"rebuild after restore","idempotency_key":"k4"}`,
}

// recoveryParityOutcomes is the ONE shared 12-row outcome matrix the
// native-vs-relay parity proof drives for all four recovery routes:
// same status, same headers, same body across the pinned outcomes in
// both negotiated shapes. Every row below must show exactly one
// upstream hit on each lane.
//
// Honesty note: the idempotency-key 409 rows (key reuse, in progress)
// are stub-imposed lane-equivalence rows for refinalize. Refinalize
// takes no idempotency key and its real core 409 is the
// blocked/claim_fence_busy refusal; the stub answers the generic
// conflict bodies on that route anyway, so those two rows prove only
// that the lanes relay bytes identically, not a core contract for
// refinalize.
var recoveryParityOutcomes = []struct {
	name   string
	status int
	header http.Header
	body   string
}{
	{"accepted 200", http.StatusOK, nil, `{"status":"accepted","detail":"recorded"}`},
	{"duplicate 200", http.StatusOK, nil, `{"status":"accepted","detail":"recorded","duplicate":true}`},
	{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"reason is required"}`},
	{"missing key 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"idempotency_key is required"}`},
	{"key reuse 409", http.StatusConflict, nil, `{"error":"Conflict","detail":"idempotency_key was already used with different parameters"}`},
	{"in progress 409", http.StatusConflict, nil, `{"error":"Conflict","detail":"a request with this idempotency_key is already in progress"}`},
	{"core 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"admin authentication is required"}`},
	{"unknown scope 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"no ingestion scope matches scope_id"}`},
	{"refused 422", http.StatusUnprocessableEntity, nil, `{"status":"refused","detail":"nothing was reopened and the idempotency key was not consumed"}`},
	{"core 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"store is unavailable"}`},
	{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
	{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the admin store is not configured"}`},
}

// TestRecoveryNative_MatchesRelayByteForByte pins that the tier C-c2
// clones relay core bytes exactly like the legacy generic proxy they
// replace: same status, same headers (modulo per-hop framing), same
// body — across the shared outcome matrix in both negotiated shapes.
// The legacy lane is replayed by mounting the generic dispatch for the
// same path in a test mux, so the comparison is native-vs-relay on the
// same binary and stub core, not a copy of the implementation. Both
// lanes must show exactly one upstream hit: dest never retries a
// forwarded mutation.
func TestRecoveryNative_MatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	for _, route := range recoveryRoutes {
		method, path, _ := strings.Cut(route, " ")
		payload, ok := recoveryPayloadByPath[path]
		if !ok {
			t.Fatalf("%s: no parity payload pinned, add it to recoveryPayloadByPath", route)
		}
		for _, outcome := range recoveryParityOutcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				core := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)

				nativeSrv := NewServer(upstream.URL, skeletonTestKey)
				nativeHandler := nativeSrv.Handler(DefaultV0Sunset)
				nativeReq := authedRequest(method, path)
				nativeReq.Header.Set("Accept", accept)
				nativeReq.Body = io.NopCloser(strings.NewReader(payload))
				nativeRec := httptest.NewRecorder()
				nativeHandler.ServeHTTP(nativeRec, nativeReq)
				nativeResult := nativeRec.Result()
				nativeBody, _ := io.ReadAll(nativeResult.Body)
				upstream.Close()

				if core.hits != 1 {
					t.Errorf("%s %s Accept %q: native upstream hits = %d, want exactly 1 (no retry on write-through)",
						route, outcome.name, accept, core.hits)
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
				relayReq.Body = io.NopCloser(strings.NewReader(payload))
				relayRec := httptest.NewRecorder()
				legacy.ServeHTTP(relayRec, relayReq)
				relayResult := relayRec.Result()
				relayBody, _ := io.ReadAll(relayResult.Body)

				if relayCore.hits != 1 {
					t.Errorf("%s %s Accept %q: relay upstream hits = %d, want exactly 1 (no retry on the legacy lane either)",
						route, outcome.name, accept, relayCore.hits)
				}
				if nativeRec.Code != relayRec.Code {
					t.Errorf("%s %s Accept %q: native status = %d, relay status = %d, want equal",
						route, outcome.name, accept, nativeRec.Code, relayRec.Code)
				}
				if string(nativeBody) != string(relayBody) {
					t.Errorf("%s %s Accept %q: native body = %q, relay body = %q, want byte-identical",
						route, outcome.name, accept, nativeBody, relayBody)
				}
				for _, key := range []string{"Retry-After", "Content-Type"} {
					if got, want := nativeResult.Header.Get(key), relayResult.Header.Get(key); got != want {
						t.Errorf("%s %s Accept %q: native %s = %q, relay %s = %q, want equal",
							route, outcome.name, accept, key, got, key, want)
					}
				}
			}
		}
	}
}
