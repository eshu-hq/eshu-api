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

// recoveryNativeCases covers the four tier C-c2 routes with concrete
// request paths: replay, reopen, refinalize, and recover-generations.
// backfill, dead-letter, reindex, and skip have no RecoveryAdmission or
// ReplayLedgerAdmission pin and stay on the generic relay (pinned by
// TestRecoveryNative_AdminFourMovedToD7Lane below); nobody infers them
// into this lane.
var recoveryNativeCases = []struct {
	name   string
	method string
	path   string
}{
	{"replay", http.MethodPost, "/api/v0/admin/replay"},
	{"reopen", http.MethodPost, "/api/v0/admin/reopen"},
	{"refinalize", http.MethodPost, "/api/v0/admin/refinalize"},
	{"recover generations", http.MethodPost, "/api/v0/admin/recover-generations"},
}

// TestRecoveryNative_UnauthenticatedIs401 pins that every native
// recovery route keeps the dest 401 shape: envelope callers get the
// envelope error, everyone else the plain error_code body, both with
// the bare Bearer challenge.
func TestRecoveryNative_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range recoveryNativeCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s Accept %q: status = %d, want 401", tc.name, accept, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("%s Accept %q: WWW-Authenticate = %q, want bare Bearer", tc.name, accept, got)
			}
		}
	}
}

// TestRecoveryNative_AuthorizerRejectsNonAllScopes pins the family
// 403 the dest authorizer adds: a non-AllScopes identity never reaches
// the core. Dest has no scoped/session resolver, so the deny branch is
// unreachable in production (the shared key maps to AllScopes in core,
// which re-judges tenant scope and the recovery permission catalog as
// backstop); the test pins the documented shape, not a live path.
func TestRecoveryNative_AuthorizerRejectsNonAllScopes(t *testing.T) {
	t.Parallel()

	if !authorizeRecoveryAdmin(true) {
		t.Errorf("authorizeRecoveryAdmin(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeRecoveryAdmin(false) {
		t.Errorf("authorizeRecoveryAdmin(false) = allow, want deny (family 403)")
	}
}

// TestRecoveryNative_DenyBranchShapePinned pins the deny shape for
// the day dest gains a scoped/session resolver: a non-AllScopes identity
// is denied with the documented shape and the core is never reached, in
// both negotiated shapes. The branch is unreachable on the live routes
// today (the routes pass true; the live 403s are core's), so this pins
// the contract, not a live path.
func TestRecoveryNative_DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, tc := range recoveryNativeCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(tc.method, tc.path)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			srv.recoveryWriteThroughAs(rec, req, false)

			if rec.Code != http.StatusForbidden {
				t.Errorf("%s Accept %q: status = %d, want 403", tc.name, accept, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), "all-scopes operator authorization is required") {
				t.Errorf("%s Accept %q: body = %q, want the family-403 message", tc.name, accept, body)
			}
		}
	}
}

// TestRecoveryNative_WriteThroughRelaysCoreBytes pins the native
// write-through: method, path, query, headers (including the original
// Authorization header), and body forward to the same core route, and
// the core's status, headers, and body come back untouched across the
// pinned outcomes — 200 results and idempotent-retry no-ops, the 400
// validation refusals, the 422 pre-claim refusals (whose single
// upstream hit proves dest never retries a refusal), the core 403s,
// the ledger 409s (in-progress and fingerprint-mismatch, whose single
// upstream hit proves dest never retries a conflict), and the core 503
// (whose Retry-After relays, proving dest never invents or strips the
// core's retry signal). Every case runs in both negotiated shapes.
//
// Core bodies below are representative wire strings where core owns
// the text (fixed WriteError refusals and ledger-conflict answers from
// the in-tree handlers at the provenance pin); the relay — not the
// text — is what this test pins.
func TestRecoveryNative_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	replayPayload := `{"scope_id":"s1","reason":"retry after the fix deployed","idempotency_key":"k1"}`
	replayForced := `{"scope_id":"s1","failure_class":"projection_bug","reason":"cause addressed","idempotency_key":"k2","force":true}`
	replayed := `{"status":"replayed","replayed_count":2,"work_item_ids":["w1","w2"],"idempotency_key":"k1","duplicate":false}`
	replayedDuplicate := `{"status":"replayed","replayed_count":2,"work_item_ids":["w1","w2"],"idempotency_key":"k1","duplicate":true}`
	reopenPayload := `{"domain":"workload_materialization","scope_id":"s1","reason":"hand repair","idempotency_key":"k3"}`
	reopened := `{"status":"reopened","domain":"workload_materialization","scope_id":"s1","generation_id":"g7","reopened_total_count":2,"idempotency_key":"k3","duplicate":false}`
	refinalizePayload := `{"scope_ids":["s1"]}`
	refinalized := `{"status":"accepted","enqueued":1,"scope_ids":["s1"]}`
	recoverPayload := `{"scope_ids":["s1"],"reason":"rebuild after restore","idempotency_key":"k4"}`
	recovered := `{"status":"recovered","enqueued":1,"scope_ids":["s1"],"idempotency_key":"k4","duplicate":false}`
	recoveredDuplicate := `{"status":"recovered","enqueued":1,"scope_ids":["s1"],"idempotency_key":"k4","duplicate":true}`
	cases := []struct {
		name    string
		route   int // index into recoveryNativeCases
		status  int
		header  http.Header
		body    string
		payload string
		query   string
	}{
		{"replay", 0, http.StatusOK, nil, replayed, replayPayload, ""},
		{"replay duplicate", 0, http.StatusOK, nil, replayedDuplicate, replayPayload, ""},
		{"replay in progress 409", 0, http.StatusConflict, nil, `{"error":"Conflict","detail":"a replay for this idempotency_key is already in progress"}`, replayPayload, ""},
		{"replay key reuse 409", 0, http.StatusConflict, nil, `{"error":"Conflict","detail":"idempotency_key was already used with different replay parameters"}`, replayPayload, ""},
		{"replay forced unsafe 200", 0, http.StatusOK, nil, replayed, replayForced, ""},
		{"replay refused 422", 0, http.StatusUnprocessableEntity, nil, `{"status":"refused","failure_class":"projection_bug","reason":"unsafe failure class","detail":"set force=true to replay this class after addressing the cause"}`, `{"failure_class":"projection_bug","reason":"retry","idempotency_key":"k5"}`, ""},
		{"replay missing reason 400", 0, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"reason is required and must explain why the replay is safe"}`, `{"scope_id":"s1","idempotency_key":"k1"}`, ""},
		{"reopen", 1, http.StatusOK, nil, reopened, reopenPayload, ""},
		{"reopen unknown scope 404", 1, http.StatusNotFound, nil, `{"error":"Not Found","detail":"no ingestion scope matches scope_id"}`, `{"domain":"workload_materialization","scope_id":"nope","reason":"hand repair","idempotency_key":"k6"}`, ""},
		{"reopen no generation 422", 1, http.StatusUnprocessableEntity, nil, `{"status":"refused","reason":"scope has no active generation to reopen work for","detail":"nothing was reopened and the idempotency key was not consumed"}`, reopenPayload, ""},
		{"refinalize", 2, http.StatusOK, nil, refinalized, refinalizePayload, ""},
		{"refinalize fence busy 409", 2, http.StatusConflict, nil, `{"error":"Conflict","detail":"blocked/claim_fence_busy"}`, refinalizePayload, ""},
		{"refinalize empty 400", 2, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"scope_ids is required and must not be empty"}`, `{}`, ""},
		{"recover generations", 3, http.StatusOK, nil, recovered, recoverPayload, ""},
		{"recover generations duplicate", 3, http.StatusOK, nil, recoveredDuplicate, recoverPayload, ""},
		{"recover generations mode conflict 400", 3, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"all_scopes cannot be combined with scope_ids: rebuild every recoverable scope or name a list, not both"}`, `{"scope_ids":["s1"],"all_scopes":true,"reason":"rebuild","idempotency_key":"k7"}`, ""},
		{"core all-scope 403", 0, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"replay requires an admin (all-scopes) token"}`, replayPayload, ""},
		{"core 503 retryable", 2, http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`, refinalizePayload, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				route := recoveryNativeCases[tc.route]
				core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, skeletonTestKey)
				handler := srv.Handler(DefaultV0Sunset)

				target := route.path
				if tc.query != "" {
					target += "?" + tc.query
				}
				req := authedRequest(route.method, target)
				req.Header.Set("Accept", accept)
				if tc.payload != "" {
					req.Body = io.NopCloser(strings.NewReader(tc.payload))
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Fatalf("%s Accept %q: upstream hits = %d, want exactly 1 (no retry on write-through)", tc.name, accept, core.hits)
				}
				if core.lastMethod != route.method {
					t.Errorf("%s Accept %q: upstream method = %q, want %s", tc.name, accept, core.lastMethod, route.method)
				}
				if core.lastPath != route.path {
					t.Errorf("%s Accept %q: upstream path = %q, want the core route %q", tc.name, accept, core.lastPath, route.path)
				}
				if core.lastQuery != tc.query {
					t.Errorf("%s Accept %q: upstream query = %q, want %q", tc.name, accept, core.lastQuery, tc.query)
				}
				if core.lastBody != tc.payload {
					t.Errorf("%s Accept %q: upstream body = %q, want the caller bytes", tc.name, accept, core.lastBody)
				}
				if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
					t.Errorf("%s Accept %q: upstream Authorization = %q, want the forwarded Bearer [REDACTED]", tc.name, accept, core.lastAuth)
				}
				if rec.Code != tc.status {
					t.Errorf("%s Accept %q: status = %d, want relayed %d", tc.name, accept, rec.Code, tc.status)
				}
				if tc.header != nil {
					if got := rec.Header().Get("Retry-After"); got != tc.header.Get("Retry-After") {
						t.Errorf("%s Accept %q: Retry-After = %q, want the core's value relayed", tc.name, accept, got)
					}
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != tc.body {
					t.Errorf("%s Accept %q: body = %q, want the core bytes %q", tc.name, accept, body, tc.body)
				}
			}
		})
	}
}

// TestRecoveryNative_DispatchViaIsNative pins the registration this
// lane exists for: the four recovery POSTs cross the explicit native
// routes ahead of the generic relay, so the dispatch signal files
// via="native" under the recovery family. A route that fell through
// to the generic proxy would file via="relay" instead, and this test
// would fail: byte-identity between the lanes is the point, so only
// the dispatch signal can tell them apart.
func TestRecoveryNative_DispatchViaIsNative(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(discardLogger())
	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range recoveryNativeCases {
		req := authedRequest(tc.method, tc.path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want relayed 200", tc.name, rec.Code)
		}
	}
	if core.hits != len(recoveryNativeCases) {
		t.Fatalf("upstream hits = %d, want %d (one write-through per route)", core.hits, len(recoveryNativeCases))
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Result().Body)
	text := string(body)
	for _, want := range []string{
		`eshu_api_dispatch_total{family="recovery",via="native",outcome="core_ok"} 4`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("/metrics missing %q\nfull body:\n%s", want, text)
		}
	}
}

// TestRecoveryNative_AdminFourMovedToD7Lane pins the D7 handoff: the
// admin-four (backfill, dead-letter, reindex, skip) no longer cross
// the generic byte-relay — they ride the D7 tier-C clone lane
// (api/d7.go) with their own pins (eshu-hq/eshu#10312). Each POST
// still forwards once with method, path, and body intact, but files
// via="native" under the admin family instead of the relay lane.
// See TestD7Native_RegisteredAheadOfRelay for the via proof; this
// test pins that the recovery lane's own posture is unchanged by the
// move.
func TestRecoveryNative_AdminFourMovedToD7Lane(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(discardLogger())
	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	for i, path := range []string{
		"/api/v0/admin/backfill",
		"/api/v0/admin/dead-letter",
		"/api/v0/admin/reindex",
		"/api/v0/admin/skip",
	} {
		req := authedRequest(http.MethodPost, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != i+1 || core.lastPath != path || core.lastMethod != http.MethodPost {
			t.Errorf("path %q: hits = %d method = %q lastPath = %q, want one forward per route",
				path, core.hits, core.lastMethod, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("path %q: status = %d, want 200", path, rec.Code)
		}
	}
	if got := tel.snapshot().dispatches["admin"][viaNative][outcomeCoreOK]; got != 4 {
		t.Errorf("admin native core_ok = %d, want 4 (the admin-four ride the D7 native lane)", got)
	}
}

// TestRecoveryNative_D3ReadsForwardIntact pins that the D3 lane moves
// these reads without changing the forwarding: the replay-events,
// work-items, and dead-letters query POSTs terminate natively since D3
// (api/d3_wave.go, family admin) with exactly one upstream hit, path,
// method, and body intact on the native lane. The admin-four
// (backfill, dead-letter, reindex, skip) moved to the D7 clone lane;
// they are unaffected above.
func TestRecoveryNative_D3ReadsForwardIntact(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/admin/replay-events/query",
		"/api/v0/admin/work-items/query",
		"/api/v0/admin/dead-letters/query",
	} {
		core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		tel := NewTelemetry(nil)
		srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != path || core.lastMethod != http.MethodPost {
			t.Errorf("path %q: hits = %d method = %q lastPath = %q, want the terminated read forwarded once untouched",
				path, core.hits, core.lastMethod, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("path %q: status = %d, want 200", path, rec.Code)
		}
		if got := tel.snapshot().dispatches["admin"][viaNative][outcomeCoreOK]; got != 1 {
			t.Errorf("path %q: admin native core_ok = %d, want 1 (D3 terminated lane)", path, got)
		}
	}
}
