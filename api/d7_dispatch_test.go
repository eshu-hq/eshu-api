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

// D7 admin-four tier-C clone dispatch proof (Refs eshu-hq/eshu#9669,
// part of #6707; pins eshu-hq/eshu#10312). The clone is HTTP
// write-through: method, path, query, headers (including the original
// Authorization header), and body forward to the same core route, and
// core bytes relay back unmodified. Dest mints no idempotency key,
// keeps no ledger row, appends no audit event, and runs no finalize —
// the claim/complete pair (where the core handler has one) runs inside
// the core handler on every forwarded request.

// d7NativeCases covers the four tier-C clone routes with concrete
// request paths: backfill, dead-letter, reindex, and skip.
var d7NativeCases = []struct {
	name   string
	method string
	path   string
}{
	{"backfill", http.MethodPost, "/api/v0/admin/backfill"},
	{"dead-letter", http.MethodPost, "/api/v0/admin/dead-letter"},
	{"reindex", http.MethodPost, "/api/v0/admin/reindex"},
	{"skip", http.MethodPost, "/api/v0/admin/skip"},
}

// TestD7Native_UnauthenticatedIs401 pins that every native admin-four
// route keeps the dest 401 shape: envelope callers get the envelope
// error, everyone else the plain error_code body, both with the bare
// Bearer challenge.
func TestD7Native_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range d7NativeCases {
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

// TestD7Native_AuthorizerRejectsNonAllScopes pins the family 403 the
// dest authorizer adds: a non-AllScopes identity never reaches the
// core. Dest has no scoped/session resolver, so the deny branch is
// unreachable in production (the shared key maps to AllScopes in core,
// which re-judges tenant scope and the mutation permission catalog as
// backstop); the test pins the documented shape, not a live path.
func TestD7Native_AuthorizerRejectsNonAllScopes(t *testing.T) {
	t.Parallel()

	if !authorizeD7Admin(true) {
		t.Errorf("authorizeD7Admin(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeD7Admin(false) {
		t.Errorf("authorizeD7Admin(false) = allow, want deny (family 403)")
	}
}

// TestD7Native_DenyBranchShapePinned pins the deny shape for the day
// dest gains a scoped/session resolver: a non-AllScopes identity is
// denied with the documented shape and the core is never reached, in
// both negotiated shapes. The branch is unreachable on the live routes
// today (the routes pass true; the live 403s are core's), so this pins
// the contract, not a live path.
func TestD7Native_DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, tc := range d7NativeCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(tc.method, tc.path)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			srv.d7WriteThroughAs(rec, req, false)

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

// TestD7Native_WriteThroughRelaysCoreBytes pins the native
// write-through: method, path, query, headers (including the original
// Authorization header), and body forward to the same core route, and
// the core's status, headers, and body come back untouched across the
// pinned outcomes — 200/202 results and recorded outcomes, duplicate
// deliveries, the 400 validation refusals, the missing-key 400 (dest
// invents no key validation: it forwards and relays core's refusal),
// the 409s (key-reuse and in-progress, whose single upstream hit proves
// dest never retries a conflict), the core 403s, the core 500, and the
// two 503 shapes (whose Retry-After relays exactly as the core sent it,
// proving dest never invents or strips the core's retry signal). Every
// case runs in both negotiated shapes.
//
// Core bodies below are representative wire strings where core owns
// the text (fixed WriteError refusals and conflict answers from the
// in-tree handlers at the provenance pin); the relay — not the
// text — is what this test pins.
func TestD7Native_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	backfillPayload := `{"reason":"rebuild after restore","idempotency_key":"k-b1"}`
	backfilled := `{"status":"recorded","request_id":"b1","idempotency_key":"k-b1"}`
	deadLetterPayload := `{"work_item_ids":["w1"],"reason":"poisoned input","idempotency_key":"k-d1"}`
	deadLettered := `{"status":"dead_lettered","count":1,"idempotency_key":"k-d1"}`
	reindexFleetPayload := `{"ingester":"repository","scope":"workspace"}`
	reindexedFleet := `{"status":"accepted","ingester":"repository","scope":"workspace","force":true,"requested_at":"2026-11-06T00:00:00Z","detail":"fleet reindex recorded"}`
	reindexRepoPayload := `{"ingester":"repository","scope":"repository","repositories":["repo-1"]}`
	reindexedRepo := `{"status":"accepted","ingester":"repository","scope":"repository","force":true,"repositories":[{"repository_id":"r1","scope_id":"repo-1","requested_at":"2026-11-06T00:00:00Z"}],"detail":"repository reindex recorded"}`
	skipPayload := `{"repository_id":"repo-1","note":"quarantine flaky shard"}`
	skipped := `{"status":"skipped","count":2}`
	cases := []struct {
		name    string
		route   int // index into d7NativeCases
		status  int
		header  http.Header
		body    string
		payload string
		query   string
	}{
		{"backfill", 0, http.StatusOK, nil, backfilled, backfillPayload, ""},
		{"backfill refused 400", 0, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"reason is required"}`, `{"idempotency_key":"k-b1"}`, ""},
		{"backfill duplicate 200", 0, http.StatusOK, nil, backfilled + `,"duplicate":true`, backfillPayload, ""},
		{"dead-letter", 1, http.StatusOK, nil, deadLettered, deadLetterPayload, ""},
		{"dead-letter refused 400", 1, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"work_item_ids is required"}`, `{"reason":"x","idempotency_key":"k-d1"}`, ""},
		{"dead-letter key reuse 409", 1, http.StatusConflict, nil, `{"error":"Conflict","detail":"idempotency_key was already used with different parameters"}`, deadLetterPayload, ""},
		{"dead-letter in progress 409", 1, http.StatusConflict, nil, `{"error":"Conflict","detail":"a request with this idempotency_key is already in progress"}`, deadLetterPayload, ""},
		{"reindex fleet 202", 2, http.StatusAccepted, nil, reindexedFleet, reindexFleetPayload, ""},
		{"reindex repository 202", 2, http.StatusAccepted, nil, reindexedRepo, reindexRepoPayload, ""},
		{"reindex refused 400", 2, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unknown repository selector: nope"}`, `{"scope":"repository","repositories":["nope"]}`, ""},
		{"skip", 3, http.StatusOK, nil, skipped, skipPayload, ""},
		{"skip ambiguous 409", 3, http.StatusConflict, nil, `{"error":"Conflict","detail":"repository_id matches several scopes"}`, `{"repository_id":"amb","note":"x"}`, ""},
		{"skip missing key 400", 0, http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"idempotency_key is required"}`, `{"reason":"no key"}`, ""},
		{"core all-scope 403", 1, http.StatusForbidden, nil, `{"error":"Forbidden","detail":"admin authentication is required"}`, deadLetterPayload, ""},
		{"core 500", 2, http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"store is unavailable"}`, reindexFleetPayload, ""},
		{"core 503 retryable", 3, http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`, skipPayload, ""},
		{"core 503 permanent", 0, http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the admin store is not configured"}`, backfillPayload, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				route := d7NativeCases[tc.route]
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
					t.Errorf("%s Accept %q: upstream Authorization = %q, want the forwarded Bearer", tc.name, accept, core.lastAuth)
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

// TestD7Native_IdempotencyKeyForwardsUntouched pins the binding key
// rule for the clone lane: dest never mints, strips, validates, or
// synthesizes an idempotency key. A payload carrying one arrives
// core-side byte-identical; a payload missing one still forwards
// exactly once and relays core's refusal. Key validity stays core's
// judgment — the ledger that makes a retried key safe is the open
// termination obligation, not dest behavior.
func TestD7Native_IdempotencyKeyForwardsUntouched(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		payload string
		wantKey string // empty means the payload carries no key
	}{
		{"key present", `{"reason":"r","idempotency_key":"k-present-1"}`, "k-present-1"},
		{"key missing", `{"reason":"r"}`, ""},
	} {
		core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, "/api/v0/admin/backfill")
		req.Body = io.NopCloser(strings.NewReader(tc.payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 {
			t.Errorf("%s: upstream hits = %d, want exactly 1", tc.name, core.hits)
		}
		if tc.wantKey != "" && !strings.Contains(core.lastBody, `"idempotency_key":"`+tc.wantKey+`"`) {
			t.Errorf("%s: upstream body = %q, want the caller key bytes intact", tc.name, core.lastBody)
		}
		if tc.wantKey == "" && strings.Contains(core.lastBody, "idempotency_key") {
			t.Errorf("%s: upstream body = %q, want no key invented by dest", tc.name, core.lastBody)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want relayed 200", tc.name, rec.Code)
		}
	}
}

// d7DuplicateStub mimics the pin contracts' current-state mutation
// semantics core-side: backfill records every POST (not even
// state-idempotent — a retried clone inserts again, disclosed in
// contracts/d7-admin-four.md); dead-letter, reindex, and skip converge
// (the second identical POST records no new effect and replays the
// stored outcome). Dest behavior under test: forward both deliveries,
// exactly one upstream hit each, no dest dedup, no dest retry.
type d7DuplicateStub struct {
	t       *testing.T
	hits    int
	effects int
	seen    map[string]string // body -> stored outcome
}

func newD7DuplicateStub(t *testing.T) *d7DuplicateStub {
	t.Helper()
	return &d7DuplicateStub{t: t, seen: make(map[string]string)}
}

func (s *d7DuplicateStub) handler(route string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			s.t.Errorf("stub read body: %v", err)
		}
		s.hits++
		// The whole body is the seen-key: identical deliveries are
		// duplicates, and the per-route branch below decides whether
		// the core converges them (dead-letter, reindex, skip) or
		// records each one (backfill, which has no ledger).
		key := string(raw)
		outcome := `{"status":"accepted","route":"` + route + `"}`
		if route == "/api/v0/admin/backfill" {
			// No ledger: every delivery is a new effect.
			s.effects++
		} else if prev, ok := s.seen[key]; ok {
			outcome = prev
		} else {
			s.effects++
			s.seen[key] = outcome
		}
		_, _ = io.WriteString(w, outcome)
	})
}

// TestD7Native_DuplicateIsNoWorseThanCore pins the tier-C proof-box
// duplicate rule: a retried clone lands exactly as it would against
// the core directly, because dest adds nothing — no dedup cache, no
// retry, no coalescing. Two identical deliveries forward twice (2
// upstream hits); the second body equals the first (core's recorded
// outcome, or core's second row for backfill, which has no ledger).
// The ledger deficit stays open in the contract: this test proves
// dest is no worse than direct core, not that a retried backfill is
// safe.
func TestD7Native_DuplicateIsNoWorseThanCore(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		route       string
		payload     string
		wantEffects int // core-side effects after two identical deliveries
	}{
		{"/api/v0/admin/backfill", `{"reason":"r","idempotency_key":"k-dup-b"}`, 2},
		{"/api/v0/admin/dead-letter", `{"work_item_ids":["w1"],"reason":"r","idempotency_key":"k-dup-d"}`, 1},
		{"/api/v0/admin/reindex", `{"scope":"workspace","idempotency_key":"k-dup-r"}`, 1},
		{"/api/v0/admin/skip", `{"repository_id":"repo-1","note":"n","idempotency_key":"k-dup-s"}`, 1},
	} {
		stub := newD7DuplicateStub(t)
		upstream := httptest.NewServer(stub.handler(tc.route))
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		var bodies []string
		for i := 0; i < 2; i++ {
			req := authedRequest(http.MethodPost, tc.route)
			req.Body = io.NopCloser(strings.NewReader(tc.payload))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s delivery %d: status = %d, want 200", tc.route, i, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			bodies = append(bodies, string(body))
		}
		if stub.hits != 2 {
			t.Errorf("%s: upstream hits = %d, want 2 (dest forwards both, dedups neither)", tc.route, stub.hits)
		}
		if bodies[0] != bodies[1] {
			t.Errorf("%s: second body = %q, want the first body %q (recorded outcome replayed)", tc.route, bodies[1], bodies[0])
		}
		if stub.effects != tc.wantEffects {
			t.Errorf("%s: core effects = %d, want %d per the pin's current-state semantics", tc.route, stub.effects, tc.wantEffects)
		}
	}
}

// TestD7Native_DispatchViaIsNative pins the registration this lane
// exists for: the four admin-four POSTs file via="native" under the
// admin family. A route that fell through to the generic proxy would
// file via="relay" instead.
func TestD7Native_DispatchViaIsNative(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(discardLogger())
	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range d7NativeCases {
		req := authedRequest(tc.method, tc.path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want relayed 200", tc.name, rec.Code)
		}
	}
	if core.hits != len(d7NativeCases) {
		t.Fatalf("upstream hits = %d, want %d (one write-through per route)", core.hits, len(d7NativeCases))
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

// TestD7Native_RecoveryStillNative pins that the C-c2 lane is
// unaffected by the D7 registrations: the replay POST still rides its
// recovery native route with exactly one upstream hit, and a D3 admin
// read still terminates on its own lane. The admin-four routes share
// no path prefix with the recovery or read routes beyond /api/v0/admin,
// and Go matches method and path together, so the lanes never merge.
func TestD7Native_RecoveryStillNative(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/admin/replay",
		"/api/v0/admin/reopen",
		"/api/v0/admin/refinalize",
		"/api/v0/admin/recover-generations",
		"/api/v0/admin/work-items/query",
		"/api/v0/admin/dead-letters/query",
	} {
		core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != path || core.lastMethod != http.MethodPost {
			t.Errorf("path %q: hits = %d method = %q lastPath = %q, want the neighboring native lane to forward once untouched",
				path, core.hits, core.lastMethod, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("path %q: status = %d, want 200", path, rec.Code)
		}
	}
}
