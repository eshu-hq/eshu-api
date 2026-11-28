// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"

	"github.com/eshu-hq/eshu-api/internal/adminread"
)

// Admin dead-letters query native termination (satellite
// eshu-hq/eshu-api#324, the second D3 store-backed promotion). TDD
// record: this suite failed RED before the lane landed (the route
// constant, the terminated handler, the store field, and the reader
// store method did not exist, so every assertion below failed at
// compile; the shared-key legs rode the generic D3 gate, so the
// zero-core assertions failed) and is green with it. This file owns
// the termination half: the freeze, the local termination, the
// required-field/bounds parity, the apilib matrix, and the zero-core
// P-READ proof. The v1 alias, census, deny branch, fail-closed
// origin, spec key, benchmark, and gate matrix live in
// api/admin_deadletters_parity_test.go.
//
// Lane rule (arbiter D-wave ruling, decision 4, via the D3 wave): the
// shared-key class terminates locally on the pinned apilib
// ServeDeadLettersQuery body served from the reader-role PG store
// with zero core contact; credential classes dest cannot resolve
// (scoped tokens, OIDC bearers, browser sessions) relay explicitly on
// the untouched D3 gate so core judges them; credential-less and
// junk-cookie callers keep the standing 401 with zero core contact.
// The only dest-synthesized errors on the terminated path are the
// gate 401, the family 403, the required-field/bounds 400s, and the
// permanent local 503 when no core origin is configured (never
// Retry-After). A missing store answers the apilib local 503 (never
// a forward); a failed store answers the fixed 500 (never the
// backend text).
//
// Required fields (core parity, preserved dest-side because the
// pinned apilib v0.2.0 defaults instead of requiring): limit
// (1-500) and timeout_ms (1-30000) are required; the bounds and the
// updated_after/updated_before RFC3339/ordering errors carry the
// core-identical detail strings. The timeout also bounds the store
// call through the request context.
//
// Proof vocabulary: P-READ carries the RunProof case — success emits
// >= 1 postgresql/read span from the production reader store
// (internal/adminread over a fake pool) through the context recorder
// with zero core-origin calls; the denial leg asserts absolute span
// silence.

// deadLettersAuthedPOST builds a shared-key dead-letters query
// request with a JSON body through the full production chain.
func deadLettersAuthedPOST(t *testing.T, target, body string) *http.Request {
	t.Helper()
	req := authedRequest(http.MethodPost, target)
	req.Body = io.NopCloser(strings.NewReader(body))
	return req
}

// deadLettersValidBody is the minimal valid query body: both
// required fields present and in bounds.
const deadLettersValidBody = `{"limit":10,"timeout_ms":5000}`

// TestAdminDeadLettersRouteFrozen pins the promoted surface: the
// single dead-letters query POST the lane terminates. A route rename
// or rehome fails here until the ownership map, the spec fragment,
// and this mirror move together.
func TestAdminDeadLettersRouteFrozen(t *testing.T) {
	t.Parallel()

	if adminDeadLettersRoute != "POST /api/v0/admin/dead-letters/query" {
		t.Fatalf("adminDeadLettersRoute = %q, want the frozen dead-letters query POST", adminDeadLettersRoute)
	}
}

// TestAdminDeadLettersTerminatesLocally pins that the promoted route
// is an explicit registration ahead of the generic proxy: a
// shared-key call serves the apilib bounded-list body on the native
// lane with zero core contact — exact path and trailing-slash twin
// alike.
func TestAdminDeadLettersTerminatesLocally(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	failure := "build_failed"
	store := &fakeDeadLetterStore{items: []apilibadmin.DeadLetterWorkItem{
		{WorkItemID: "w1", ScopeID: "s1", GenerationID: "g1", Stage: "build", Domain: "d", CollectorKind: "collector-1", AttemptCount: 3, FailureClass: &failure},
	}}

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel).WithDeadLetterStore(store)
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/admin/dead-letters/query",
		"/api/v0/admin/dead-letters/query/",
	} {
		req := deadLettersAuthedPOST(t, target, deadLettersValidBody)
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

// TestAdminDeadLettersRequiredFieldsParity pins the preserved
// required-field validation: limit (1-500) and timeout_ms (1-30000)
// are required with the core-identical detail strings, in both
// negotiated shapes, with zero store contact and zero core contact.
func TestAdminDeadLettersRequiredFieldsParity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		body   string
		detail string
	}{
		{"missing limit", `{"timeout_ms":5000}`, "limit is required and must be between 1 and 500"},
		{"zero limit", `{"limit":0,"timeout_ms":5000}`, "limit is required and must be between 1 and 500"},
		{"negative limit", `{"limit":-3,"timeout_ms":5000}`, "limit is required and must be between 1 and 500"},
		{"limit over bound", `{"limit":501,"timeout_ms":5000}`, "limit must be <= 500"},
		{"missing timeout", `{"limit":10}`, "timeout_ms is required and must be between 1 and 30000"},
		{"zero timeout", `{"limit":10,"timeout_ms":0}`, "timeout_ms is required and must be between 1 and 30000"},
		{"timeout over bound", `{"limit":10,"timeout_ms":30001}`, "timeout_ms must be <= 30000"},
		{"malformed updated_after", `{"limit":10,"timeout_ms":5000,"updated_after":"not-a-time"}`, "updated_after must be RFC3339"},
		{"malformed updated_before", `{"limit":10,"timeout_ms":5000,"updated_before":"yesterday"}`, "updated_before must be RFC3339"},
		{"inverted window", `{"limit":10,"timeout_ms":5000,"updated_after":"2026-03-05T00:00:00Z","updated_before":"2026-03-04T00:00:00Z"}`, "updated_after must be before updated_before"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			store := &fakeDeadLetterStore{}
			srv := NewServer(upstream.URL, skeletonTestKey).WithDeadLetterStore(store)
			counter := InstallCoreCallCounter(srv)
			handler := srv.Handler(DefaultV0Sunset)

			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				req := deadLettersAuthedPOST(t, "/api/v0/admin/dead-letters/query", tc.body)
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusBadRequest {
					t.Errorf("%s Accept %q: status = %d, want 400", tc.name, accept, rec.Code)
				}
				if got := deadLettersErrorMessage(t, rec.Body.Bytes()); got != tc.detail {
					t.Errorf("%s Accept %q: detail = %q, want %q", tc.name, accept, got, tc.detail)
				}
			}
			if store.calls != 0 {
				t.Errorf("%s: store calls = %d, want 0 (validation fails before the store)", tc.name, store.calls)
			}
			if core.hits != 0 {
				t.Errorf("%s: upstream hits = %d, want 0 (validation never forwards)", tc.name, core.hits)
			}
			counter.AssertZero(t, "required-field validation")
		})
	}
}

// TestAdminDeadLettersBoundedListParity pins the local matrix: bad
// JSON answers 400, a missing store answers the local 503 (never a
// forward), a failed store answers the fixed 500 with no backend
// text, and success answers the {count, items} shape with core
// field/RFC3339/pointer-omission parity.
func TestAdminDeadLettersBoundedListParity(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	serve := func(store apilibadmin.DeadLetterStore, body string) *httptest.ResponseRecorder {
		srv := NewServer(upstream.URL, skeletonTestKey).WithDeadLetterStore(store)
		rec := httptest.NewRecorder()
		srv.Handler(DefaultV0Sunset).ServeHTTP(rec, deadLettersAuthedPOST(t, "/api/v0/admin/dead-letters/query", body))
		return rec
	}

	t.Run("bad JSON is 400", func(t *testing.T) {
		t.Parallel()
		if rec := serve(&fakeDeadLetterStore{}, `{`); rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("missing store is local 503 never a forward", func(t *testing.T) {
		t.Parallel()
		rec := serve(nil, deadLettersValidBody)
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
		rec := serve(&fakeDeadLetterStore{err: errors.New("sql: connection refused host=db-internal password=hunter2")}, deadLettersValidBody)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		var doc map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if doc["detail"] != "dead-letter query failed" {
			t.Errorf("detail = %v, want the fixed message", doc)
		}
		if strings.Contains(rec.Body.String(), "hunter2") {
			t.Errorf("backend text leaked into body: %s", rec.Body.String())
		}
	})

	t.Run("success shape mirrors core", func(t *testing.T) {
		t.Parallel()
		failure := "build_failed"
		created := parseDeadLettersTime(t, "2026-03-04T05:06:07Z")
		updated := parseDeadLettersTime(t, "2026-03-04T06:06:07Z")
		visible := parseDeadLettersTime(t, "2026-03-04T07:06:07Z")
		store := &fakeDeadLetterStore{items: []apilibadmin.DeadLetterWorkItem{
			{
				WorkItemID: "w1", ScopeID: "s1", GenerationID: "g1",
				Stage: "build", Domain: "d", CollectorKind: "collector-1",
				AttemptCount: 3, FailureClass: &failure,
				CreatedAt: created, UpdatedAt: updated, VisibleAt: &visible,
			},
			{
				WorkItemID: "w2", ScopeID: "s1", GenerationID: "g1",
				Stage: "build", Domain: "d", CollectorKind: "collector-1",
				AttemptCount: 1, CreatedAt: created, UpdatedAt: updated,
			},
		}}
		rec := serve(store, deadLettersValidBody)
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
		if first["work_item_id"] != "w1" || first["collector_kind"] != "collector-1" || first["failure_class"] != "build_failed" {
			t.Errorf("first row = %v, want row fields with pointers", first)
		}
		if first["created_at"] != "2026-03-04T05:06:07Z" || first["updated_at"] != "2026-03-04T06:06:07Z" || first["visible_at"] != "2026-03-04T07:06:07Z" {
			t.Errorf("first row timestamps = %v, want RFC3339", first)
		}
		for _, key := range []string{"failure_class", "visible_at"} {
			if _, present := doc.Items[1][key]; present {
				t.Errorf("second row keeps nil pointer %q: %v", key, doc.Items[1])
			}
		}
	})

	t.Run("trimmed selectors pass through", func(t *testing.T) {
		t.Parallel()
		store := &fakeDeadLetterStore{}
		rec := serve(store, `{"failure_class":" fc ","domain":" d ","scope_id":" s ","collector_kind":" ck ","limit":10,"timeout_ms":5000}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		if store.calls != 1 {
			t.Fatalf("store calls = %d, want 1", store.calls)
		}
		if store.got.Limit != 10 {
			t.Errorf("limit = %d, want the caller limit 10 (no silent default)", store.got.Limit)
		}
		if store.got.FailureClass != "fc" || store.got.Domain != "d" || store.got.ScopeID != "s" || store.got.CollectorKind != "ck" {
			t.Errorf("filter = %+v, want trimmed selectors", store.got)
		}
	})
}

// TestAdminDeadLettersProofRead is the zero-core proof for the
// promoted reader: a P-READ case pairing the shared-key success
// (which must emit >= 1 postgresql/read span from the production
// store path) with the credential-less denial (absolute span
// silence) through the full production chain with the counting
// transport. The store is the real internal/adminread reader over a
// fake pool — the fake DeadLetterStore emits no spans, so the
// success span can only come from the production
// ListDeadLetterWorkItems. Every leg asserts zero core-origin calls.
func TestAdminDeadLettersProofRead(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 3, 4, 6, 6, 7, 0, time.UTC)
	store := adminread.NewStoreFromPool(&fakeDeadLetterPool{rows: &fakeDeadLetterRows{staged: true, at: at}})
	srv := NewServer("http://core.invalid", skeletonTestKey).WithDeadLetterStore(store)
	counter := InstallCoreCallCounter(srv)
	rec := &RecordingStoreSpans{}
	handler := withStoreSpanRecorder(rec, srv.Handler(DefaultV0Sunset))

	RunProof(t, handler, counter, rec, []ProofCase{
		{
			Name: "dead-letters local success",
			Kind: ProofRead,
			Requests: []ProofRequest{
				{Method: http.MethodPost, Path: "/api/v0/admin/dead-letters/query", Credential: "valid", Body: deadLettersValidBody},
			},
			WantStatuses: []int{http.StatusOK},
			WantSpans: []StoreSpanExpectation{
				{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Min: 1, Max: 1},
			},
		},
		{
			Name: "dead-letters denial silence",
			Kind: ProofRead,
			Requests: []ProofRequest{
				{Method: http.MethodPost, Path: "/api/v0/admin/dead-letters/query", Credential: "missing", Body: deadLettersValidBody},
			},
			WantStatuses: []int{http.StatusUnauthorized},
			WantSpans:    nil,
		},
	})
}

// deadLettersErrorMessage extracts the caller-visible error detail
// from either negotiated shape (the envelope nests it under error,
// the plain shape carries it as message), decoding the JSON so
// HTML-escaped bytes compare canonical.
func deadLettersErrorMessage(t *testing.T, body []byte) string {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("error body is not JSON: %v", err)
	}
	if nested, ok := doc["error"].(map[string]any); ok {
		if message, ok := nested["message"].(string); ok {
			return message
		}
	}
	if message, ok := doc["message"].(string); ok {
		return message
	}
	t.Fatalf("error body carries no message: %v", doc)
	return ""
}

// parseDeadLettersTime parses one RFC3339 instant for the parity
// fixtures. The apilib writer renders timestamps in RFC3339, so the
// fixtures pin the same rendering the success-shape case asserts.
func parseDeadLettersTime(t *testing.T, value string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("bad fixture instant %q: %v", value, err)
	}
	return at
}
