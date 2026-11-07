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

// D7 admin-four tier-C dispatch clones (Refs eshu-hq/eshu#9669, part of
// #6707). TDD record: this suite failed RED before the implementation
// landed (frozen route set asserted first, referencing the undefined
// d7Routes table and d7WriteThroughAs handler) and is green with it.
// Until the lane lands, the four routes cross the generic byte-relay,
// so the ahead-of-relay assertions fail first. That red is the recorded
// expectation, not a defect.
//
// Scope honesty: these are tier-C dispatch clones — HTTP write-through
// to the current core handlers — not termination and not removal
// readiness. The D7 pins (Backfill, DeadLetter, Reindex, Skip v0,
// eshu-hq/eshu#10312) are the recorded preconditions; the ledger,
// client-stable keys, and finalize/audit stack stay termination
// obligations for the later lane.

// TestD7RoutesFrozen pins the exact admin-four surface the satellite
// must clone. A route added, removed, or renamed fails here until the
// pin contract, the spec fragment, and this mirror move together.
func TestD7RoutesFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/admin/backfill",
		"POST /api/v0/admin/dead-letter",
		"POST /api/v0/admin/reindex",
		"POST /api/v0/admin/skip",
	}
	got := append([]string(nil), d7Routes...)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("d7Routes holds %d entries %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("d7Routes differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestD7Native_RegisteredAheadOfRelay pins the registration this lane
// exists for: the four admin-four POSTs cross the explicit native
// routes ahead of the generic relay, so the dispatch signal files
// via="native" under the admin family. A route that fell through to
// the generic proxy would file via="relay" instead, and this test
// would fail: byte-identity between the lanes is the point, so only
// the dispatch signal can tell them apart.
func TestD7Native_RegisteredAheadOfRelay(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(discardLogger())
	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	for _, route := range d7Routes {
		method, path, _ := strings.Cut(route, " ")
		req := authedRequest(method, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want relayed 200", route, rec.Code)
		}
	}
	if core.hits != len(d7Routes) {
		t.Fatalf("upstream hits = %d, want %d (one write-through per route)", core.hits, len(d7Routes))
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Result().Body)
	text := string(body)
	for _, want := range []string{
		`eshu_api_dispatch_total{family="admin",via="native",outcome="core_ok"} 4`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("/metrics missing %q\nfull body:\n%s", want, text)
		}
	}
}

// d7ParityOutcomes is the shared outcome matrix the native-vs-relay
// parity proof drives: the stub answers the pinned wire strings where
// core owns the text, and opaque bytes where the relay (not the text)
// is what the test pins. The 500/503 rows prove the dest never
// retries an unstable core: every row below must show exactly one
// upstream hit on the native lane.
var d7ParityOutcomes = []struct {
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
	{"core 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"store is unavailable"}`},
	{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
	{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the admin store is not configured"}`},
}

// TestD7Native_MatchesRelayByteForByte pins that the tier-C clone
// relays core bytes exactly like the legacy generic proxy it replaces:
// same status, same headers (modulo per-hop framing), same body —
// across the pinned outcome matrix in both negotiated shapes. The
// legacy lane is replayed by mounting the generic dispatch for the
// same path in a test mux, so the comparison is native-vs-relay on the
// same binary and stub core, not a copy of the implementation.
func TestD7Native_MatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	payload := `{"scope_id":"s1","reason":"rebuild after restore","idempotency_key":"k-d7"}`
	for _, route := range d7Routes {
		method, path, _ := strings.Cut(route, " ")
		for _, outcome := range d7ParityOutcomes {
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
