// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// Admin work-items query native termination (satellite
// eshu-hq/eshu-api#335, the first D3 Gap store-backed promotion). TDD
// record: this suite failed RED before the lane landed (the route
// constant, the terminated handler, the store field, and the reader
// store did not exist, so every assertion below failed at compile;
// the shared-key legs rode the generic D3 gate, so the zero-core
// assertions failed) and is green with it.
//
// Lane rule (arbiter D-wave ruling, decision 4, via the D3 wave): the
// shared-key class terminates locally on the pinned apilib
// ServeWorkItemsQuery body served from the reader-role PG store with
// zero core contact; credential classes dest cannot resolve (scoped
// tokens, OIDC bearers, browser sessions) relay explicitly on the
// untouched D3 gate so core judges them; credential-less and
// junk-cookie callers keep the standing 401 with zero core contact.
// The only dest-synthesized errors on the terminated path are the
// gate 401, the family 403, and the permanent local 503 when no core
// origin is configured (never Retry-After). A missing store answers
// the apilib local 503 (never a forward); a failed store answers the
// fixed 500 (never the backend text).
//
// Proof vocabulary: P-READ carries the RunProof case — success emits
// >= 1 postgresql/read span through the context recorder with zero
// core-origin calls; the denial leg asserts absolute span silence.

// fakeWorkItemStore is the in-memory apilib WorkItemStore the lane
// proofs run against: it records the filter and call count, answers
// canned rows or a canned error, and emits one postgresql/read span
// per ListWorkItems through the context recorder — the same seam the
// production reader store reports through, so the P-READ span is the
// store-path evidence, not harness theater.
type fakeWorkItemStore struct {
	items []apilibadmin.WorkItem
	err   error
	got   apilibadmin.WorkItemFilter
	calls int
}

func (f *fakeWorkItemStore) ListWorkItems(ctx context.Context, filter apilibadmin.WorkItemFilter) ([]apilibadmin.WorkItem, error) {
	f.got = filter
	f.calls++
	RecordStoreSpan(ctx, StoreSpan{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Name: "admin work-items list"})
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

// workItemsAuthedPOST builds a shared-key work-items query request
// with a JSON body through the full production chain.
func workItemsAuthedPOST(t *testing.T, target, body string) *http.Request {
	t.Helper()
	req := authedRequest(http.MethodPost, target)
	req.Body = io.NopCloser(strings.NewReader(body))
	return req
}

// TestAdminWorkItemsRouteFrozen pins the promoted surface: the single
// work-items query POST the lane terminates. A route rename or rehome
// fails here until the ownership map, the spec fragment, and this
// mirror move together.
func TestAdminWorkItemsRouteFrozen(t *testing.T) {
	t.Parallel()

	if adminWorkItemsRoute != "POST /api/v0/admin/work-items/query" {
		t.Fatalf("adminWorkItemsRoute = %q, want the frozen work-items query POST", adminWorkItemsRoute)
	}
}

// TestAdminWorkItemsTerminatesLocally pins that the promoted route is
// an explicit registration ahead of the generic proxy: a shared-key
// call serves the apilib bounded-list body on the native lane with
// zero core contact — exact path and trailing-slash twin alike.
func TestAdminWorkItemsTerminatesLocally(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	store := &fakeWorkItemStore{items: []apilibadmin.WorkItem{
		{WorkItemID: "w1", ScopeID: "s1", GenerationID: "g1", Stage: "build", Domain: "d", Status: "pending", AttemptCount: 3},
	}}

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel).WithWorkItemStore(store)
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/admin/work-items/query",
		"/api/v0/admin/work-items/query/",
	} {
		req := workItemsAuthedPOST(t, target, `{"limit":10}`)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s: status = %d, want the terminated 200", target, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Errorf("POST %s: Content-Type = %q, want the JSON document shape", target, got)
		}
		var doc struct {
			Count int `json:"count"`
			Items []struct {
				WorkItemID string `json:"work_item_id"`
			} `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("POST %s: body is not JSON: %v", target, err)
		}
		if doc.Count != 1 || len(doc.Items) != 1 || doc.Items[0].WorkItemID != "w1" {
			t.Errorf("POST %s: body = %s, want the one store row", target, rec.Body.String())
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (shared-key terminates locally)", core.hits)
	}
	counter.AssertZero(t, "shared-key local termination")
	if store.calls != 2 {
		t.Errorf("store calls = %d, want 2 (exact path plus twin)", store.calls)
	}
	snap := tel.snapshot()
	if got := snap.authorizer["admin"][decisionAllow]; got != 2 {
		t.Errorf("authorizer[admin][allow] = %d, want 2 (exact path plus twin)", got)
	}
	lanes, ok := snap.dispatches["admin"]
	if !ok {
		t.Fatal("family admin has no dispatch rows, want the explicit promoted route ahead of relay")
	}
	if got := lanes[viaNative][outcomeLocalOK]; got != 2 {
		t.Errorf("family admin native local_ok = %d, want 2 (exact path plus twin)", got)
	}
	if got := snap.requests["admin"]["2xx"]; got != 2 {
		t.Errorf("requests[admin][2xx] = %d, want 2", got)
	}
}

// TestAdminWorkItemsBoundedListParity pins the local matrix against
// the #456 consumer proof values: malformed JSON answers 400, a
// missing store answers the local 503 (never a forward), a failed
// store answers the fixed 500 with no backend text, and success
// answers the {count, items} shape with core field/RFC3339/
// pointer-omission parity.
func TestAdminWorkItemsBoundedListParity(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	serve := func(store apilibadmin.WorkItemStore, body string) *httptest.ResponseRecorder {
		srv := NewServer(upstream.URL, skeletonTestKey).WithWorkItemStore(store)
		rec := httptest.NewRecorder()
		srv.Handler(DefaultV0Sunset).ServeHTTP(rec, workItemsAuthedPOST(t, "/api/v0/admin/work-items/query", body))
		return rec
	}

	t.Run("bad JSON is 400", func(t *testing.T) {
		t.Parallel()
		if rec := serve(&fakeWorkItemStore{}, `{`); rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("missing store is local 503 never a forward", func(t *testing.T) {
		t.Parallel()
		rec := serve(nil, `{"limit":10}`)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		var doc map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if doc["error"] != "Service Unavailable" || doc["detail"] != "admin store not configured" {
			t.Errorf("body = %v, want the apilib store-absent envelope", doc)
		}
		if core.hits != 0 {
			t.Errorf("upstream hits = %d, want 0 (store-down never forwards)", core.hits)
		}
	})

	t.Run("store error is fixed 500", func(t *testing.T) {
		t.Parallel()
		rec := serve(&fakeWorkItemStore{err: errors.New("sql: connection refused host=db-internal password=hunter2")}, `{}`)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		var doc map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if doc["detail"] != "work-item query failed" {
			t.Errorf("detail = %v, want the fixed message", doc["detail"])
		}
		if strings.Contains(rec.Body.String(), "hunter2") {
			t.Errorf("backend text leaked into body: %s", rec.Body.String())
		}
	})

	t.Run("success shape mirrors core", func(t *testing.T) {
		t.Parallel()
		lease := "owner-1"
		failure := "build_failed"
		created := parseWorkItemsTime(t, "2026-03-04T05:06:07Z")
		updated := parseWorkItemsTime(t, "2026-03-04T06:06:07Z")
		visible := parseWorkItemsTime(t, "2026-03-04T07:06:07Z")
		store := &fakeWorkItemStore{items: []apilibadmin.WorkItem{
			{
				WorkItemID: "w1", ScopeID: "s1", GenerationID: "g1",
				Stage: "build", Domain: "d", Status: "pending", AttemptCount: 3,
				LeaseOwner: &lease, FailureClass: &failure,
				CreatedAt: created, UpdatedAt: updated, VisibleAt: &visible,
			},
			{
				WorkItemID: "w2", ScopeID: "s1", GenerationID: "g1",
				Stage: "build", Domain: "d", Status: "failed", AttemptCount: 1,
				CreatedAt: created, UpdatedAt: updated,
			},
		}}
		rec := serve(store, `{"limit":10}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		var doc struct {
			Count int              `json:"count"`
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if doc.Count != 2 || len(doc.Items) != 2 {
			t.Fatalf("count/items = %d/%d, want 2/2 (%s)", doc.Count, len(doc.Items), rec.Body.String())
		}
		first := doc.Items[0]
		if first["work_item_id"] != "w1" || first["lease_owner"] != "owner-1" || first["failure_class"] != "build_failed" {
			t.Errorf("first row = %v, want row fields with pointers", first)
		}
		if first["created_at"] != "2026-03-04T05:06:07Z" || first["updated_at"] != "2026-03-04T06:06:07Z" || first["visible_at"] != "2026-03-04T07:06:07Z" {
			t.Errorf("first row timestamps = %v, want RFC3339", first)
		}
		for _, key := range []string{"lease_owner", "failure_class", "failure_message", "operator_note", "prior_failure", "visible_at"} {
			if _, present := doc.Items[1][key]; present {
				t.Errorf("second row keeps nil pointer %q: %v", key, doc.Items[1])
			}
		}
	})

	t.Run("default limit 100 with trimmed selectors", func(t *testing.T) {
		t.Parallel()
		store := &fakeWorkItemStore{}
		rec := serve(store, `{"scope_id":"  s1  ","stage":" st ","failure_class":" fc "}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		if store.calls != 1 {
			t.Fatalf("store calls = %d, want 1", store.calls)
		}
		if store.got.Limit != 100 {
			t.Errorf("limit = %d, want the default 100", store.got.Limit)
		}
		if store.got.ScopeID != "s1" || store.got.Stage != "st" || store.got.FailureClass != "fc" {
			t.Errorf("filter = %+v, want trimmed selectors", store.got)
		}
	})
}

// TestAdminWorkItemsProofRead is the zero-core proof for the promoted
// reader: a P-READ case pairing the shared-key success (which must
// emit >= 1 postgresql/read span from the store path) with the
// credential-less denial (absolute span silence) through the full
// production chain with the counting transport. Every leg asserts zero
// core-origin calls.
func TestAdminWorkItemsProofRead(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey).WithWorkItemStore(&fakeWorkItemStore{})
	counter := InstallCoreCallCounter(srv)
	rec := &RecordingStoreSpans{}
	handler := withStoreSpanRecorder(rec, srv.Handler(DefaultV0Sunset))

	RunProof(t, handler, counter, rec, []ProofCase{
		{
			Name: "work-items local success",
			Kind: ProofRead,
			Requests: []ProofRequest{
				{Method: http.MethodPost, Path: "/api/v0/admin/work-items/query", Credential: "valid", Body: `{"limit":10}`},
			},
			WantStatuses: []int{http.StatusOK},
			WantSpans: []StoreSpanExpectation{
				{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Min: 1, Max: 1},
			},
		},
		{
			Name: "work-items denial silence",
			Kind: ProofRead,
			Requests: []ProofRequest{
				{Method: http.MethodPost, Path: "/api/v0/admin/work-items/query", Credential: "missing", Body: `{"limit":10}`},
			},
			WantStatuses: []int{http.StatusUnauthorized},
			WantSpans:    nil,
		},
	})
}

// TestAdminWorkItemsDenial401ZeroCore pins that the promoted route
// keeps the dest 401 shape: credential-less callers get the envelope
// error or the plain error_code body with the bare Bearer challenge
// on both spellings — and the core is never contacted.
func TestAdminWorkItemsDenial401ZeroCore(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey).WithWorkItemStore(&fakeWorkItemStore{})
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/admin/work-items/query",
		"/api/v0/admin/work-items/query/",
	} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{"limit":10}`))
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("POST %s Accept %q: status = %d, want 401", target, accept, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("POST %s Accept %q: WWW-Authenticate = %q, want bare Bearer", target, accept, got)
			}
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (gate denials never dispatch)", core.hits)
	}
	counter.AssertZero(t, "credential-less denial")
}

// TestAdminWorkItemsForeignRelayExplicit pins the never-invent rule
// for the promoted route: a call carrying a credential class dest
// cannot resolve relays explicitly to the core on the relay lane —
// core judges, core bytes come back untouched — and never answers a
// dest-invented failure.
func TestAdminWorkItemsForeignRelayExplicit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		header map[string]string
	}{
		{"scoped bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
		{"oidc bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
		{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
		{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel).WithWorkItemStore(&fakeWorkItemStore{})
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, "/api/v0/admin/work-items/query", strings.NewReader(`{"limit":10}`))
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Errorf("%s: upstream hits = %d, want 1 (relay explicitly, never gate-invent)", tc.name, core.hits)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("%s: status = %d, want the relayed core 200", tc.name, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != core.body {
				t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, body, core.body)
			}
			if got := tel.snapshot().dispatches["admin"][viaRelay][outcomeCoreOK]; got != 1 {
				t.Errorf("%s: family admin relay core_ok = %d, want 1", tc.name, got)
			}
		})
	}
}

// TestAdminWorkItemsJunkCookieStays401 pins the D2 follow-up cookie
// lesson for the promoted route: only session-cookie-shaped
// credentials relay. Any other Cookie stays dest-401 with zero core
// contact, exactly like a credential-less caller, in both negotiated
// shapes.
func TestAdminWorkItemsJunkCookieStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey).WithWorkItemStore(&fakeWorkItemStore{})
	handler := srv.Handler(DefaultV0Sunset)

	for _, cookie := range []string{
		"theme=dark",
		"unrelated=ignore; other=1",
		"eshu_session_stale=",
	} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(http.MethodPost, "/api/v0/admin/work-items/query", strings.NewReader(`{"limit":10}`))
			req.Header.Set("Accept", accept)
			req.Header.Set("Cookie", cookie)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("cookie %q Accept %q: status = %d, want 401 (junk cookies never relay)", cookie, accept, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("cookie %q Accept %q: WWW-Authenticate = %q, want bare Bearer", cookie, accept, got)
			}
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (junk cookies never reach the core)", core.hits)
	}
}
