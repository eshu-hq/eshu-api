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

// Admin decisions query native termination (satellite
// eshu-hq/eshu-api#325, the third D3 store-backed promotion). TDD
// record: this suite failed RED before the lane landed (the route
// constant, the terminated handler, the store field, and the reader
// store methods did not exist, so every assertion below failed at
// compile; the shared-key legs rode the generic D3 gate, so the
// zero-core assertions failed) and is green with it. This file owns
// the termination half: the freeze, the local termination, the
// required-field parity, the apilib matrix, and the zero-core P-READ
// proof. The v1 alias, census, deny branch, fail-closed origin, spec
// key, benchmark, and gate matrix live in
// api/admin_decisions_parity_test.go.
//
// Lane rule (arbiter D-wave ruling, decision 4, via the D3 wave): the
// shared-key class terminates locally on the pinned apilib
// ServeDecisionsQuery body served from the reader-role PG store
// with zero core contact; credential classes dest cannot resolve
// (scoped tokens, OIDC bearers, browser sessions) relay explicitly on
// the untouched D3 gate so core judges them; credential-less and
// junk-cookie callers keep the standing 401 with zero core contact.
// The only dest-synthesized errors on the terminated path are the
// gate 401, the family 403, the required-field 400s, and the
// permanent local 503 when no core origin is configured (never
// Retry-After). A missing store answers the apilib local 503 (never
// a forward); a failed store answers the fixed 500 (never the
// backend text).
//
// Required fields (core parity, served by the pinned apilib v0.3.0):
// repository_id and source_run_id are required with the
// core-identical detail; an absent limit defaults to 100, like core.
//
// Proof vocabulary: P-READ carries the RunProof case — success emits
// >= 1 postgresql/read span from the production reader store
// (internal/adminread over a fake pool) through the context recorder
// with zero core-origin calls; the denial leg asserts absolute span
// silence.

// decisionsAuthedPOST builds a shared-key decisions query request
// with a JSON body through the full production chain.
func decisionsAuthedPOST(t *testing.T, target, body string) *http.Request {
	t.Helper()
	req := authedRequest(http.MethodPost, target)
	req.Body = io.NopCloser(strings.NewReader(body))
	return req
}

// decisionsValidBody is the minimal valid query body: both required
// fields present, with the evidence fan-out on so the proof covers
// the joined leg.
const decisionsValidBody = `{"repository_id":"r1","source_run_id":"s1","limit":10,"include_evidence":true}`

// TestAdminDecisionsRouteFrozen pins the promoted surface: the
// single decisions query POST the lane terminates. A route rename
// or rehome fails here until the ownership map, the spec fragment,
// and this mirror move together.
func TestAdminDecisionsRouteFrozen(t *testing.T) {
	t.Parallel()

	if adminDecisionsRoute != "POST /api/v0/admin/decisions/query" {
		t.Fatalf("adminDecisionsRoute = %q, want the frozen decisions query POST", adminDecisionsRoute)
	}
}

// TestAdminDecisionsTerminatesLocally pins that the promoted route
// is an explicit registration ahead of the generic proxy: a
// shared-key call serves the apilib decisions body on the native
// lane with zero core contact — exact path and trailing-slash twin
// alike.
func TestAdminDecisionsTerminatesLocally(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	store := &fakeDecisionStore{decisions: []apilibadmin.DecisionRow{
		{DecisionID: "d1", DecisionType: "merge", RepositoryID: "r1", SourceRunID: "s1"},
	}}

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel).WithDecisionStore(store)
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/admin/decisions/query",
		"/api/v0/admin/decisions/query/",
	} {
		req := decisionsAuthedPOST(t, target, decisionsValidBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s: status = %d, want the terminated 200", target, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Errorf("POST %s: Content-Type = %q, want the JSON document shape", target, got)
		}
		var doc struct {
			Count     int `json:"count"`
			Decisions []struct {
				DecisionID string `json:"decision_id"`
			} `json:"decisions"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("POST %s: body is not JSON: %v", target, err)
		}
		if doc.Count != 1 || len(doc.Decisions) != 1 || doc.Decisions[0].DecisionID != "d1" {
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

// TestAdminDecisionsRequiredFieldsParity pins the required-field
// validation: repository_id and source_run_id are required with the
// core-identical detail, in both negotiated shapes, with zero store
// contact and zero core contact. An absent limit defaults to 100,
// like core — never a 400.
func TestAdminDecisionsRequiredFieldsParity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		body   string
		detail string
	}{
		{"missing pair", `{"limit":10}`, "repository_id and source_run_id are required"},
		{"missing run", `{"repository_id":"r1","limit":10}`, "repository_id and source_run_id are required"},
		{"missing repository", `{"source_run_id":"s1","limit":10}`, "repository_id and source_run_id are required"},
		{"blank pair", `{"repository_id":"  ","source_run_id":"  ","limit":10}`, "repository_id and source_run_id are required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			store := &fakeDecisionStore{}
			srv := NewServer(upstream.URL, skeletonTestKey).WithDecisionStore(store)
			counter := InstallCoreCallCounter(srv)
			handler := srv.Handler(DefaultV0Sunset)

			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				req := decisionsAuthedPOST(t, "/api/v0/admin/decisions/query", tc.body)
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusBadRequest {
					t.Errorf("%s Accept %q: status = %d, want 400", tc.name, accept, rec.Code)
				}
				if got := decisionsErrorMessage(t, rec.Body.Bytes()); got != tc.detail {
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

// TestAdminDecisionsBoundedListParity pins the local matrix: bad
// JSON answers 400, a missing store answers the local 503 (never a
// forward), a failed decision step answers the fixed 500 with no
// backend text, a failed evidence step answers its own fixed 500,
// and success answers the {count, decisions} shape with core
// field/RFC3339/pointer-omission parity plus the evidence fan-out.
func TestAdminDecisionsBoundedListParity(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	serve := func(store apilibadmin.DecisionStore, body string) *httptest.ResponseRecorder {
		srv := NewServer(upstream.URL, skeletonTestKey).WithDecisionStore(store)
		rec := httptest.NewRecorder()
		srv.Handler(DefaultV0Sunset).ServeHTTP(rec, decisionsAuthedPOST(t, "/api/v0/admin/decisions/query", body))
		return rec
	}

	t.Run("bad JSON is 400", func(t *testing.T) {
		t.Parallel()
		if rec := serve(&fakeDecisionStore{}, `{`); rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("missing store is local 503 never a forward", func(t *testing.T) {
		t.Parallel()
		rec := serve(nil, decisionsValidBody)
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

	t.Run("decision error is fixed 500", func(t *testing.T) {
		t.Parallel()
		rec := serve(&fakeDecisionStore{err: errors.New("sql: connection refused host=db-internal password=hunter2")}, decisionsValidBody)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		var doc map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if doc["detail"] != "projection decision query failed" {
			t.Errorf("detail = %v, want the fixed message", doc)
		}
		if strings.Contains(rec.Body.String(), "hunter2") {
			t.Errorf("backend text leaked into body: %s", rec.Body.String())
		}
	})

	t.Run("unknown decision evidence error is fixed 500", func(t *testing.T) {
		t.Parallel()
		store := &fakeDecisionStore{
			decisions: []apilibadmin.DecisionRow{{DecisionID: "d-missing"}},
			evErr:     errors.New("sql: connection refused host=db-internal password=hunter2"),
		}
		rec := serve(store, decisionsValidBody)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		var doc map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if doc["detail"] != "projection decision evidence query failed" {
			t.Errorf("detail = %v, want the fixed evidence message", doc)
		}
		if strings.Contains(rec.Body.String(), "hunter2") {
			t.Errorf("backend text leaked into body: %s", rec.Body.String())
		}
	})

	t.Run("success joins evidence with core parity", func(t *testing.T) {
		t.Parallel()
		created := parseDecisionsTime(t, "2026-03-04T05:06:07Z")
		fact := "f1"
		store := &fakeDecisionStore{
			decisions: []apilibadmin.DecisionRow{
				{
					DecisionID: "d1", DecisionType: "merge", RepositoryID: "r1", SourceRunID: "s1",
					WorkItemID: "w1", Subject: "subject-a", ConfidenceScore: 0.9,
					ConfidenceReason: "reason-a", ProvenanceSummary: map[string]any{"planner": "p"},
					CreatedAt: created,
				},
				{
					DecisionID: "d2", DecisionType: "split", RepositoryID: "r1", SourceRunID: "s1",
					CreatedAt: created,
				},
			},
			evidence: map[string][]apilibadmin.EvidenceRow{
				"d1": {
					{EvidenceID: "e1", DecisionID: "d1", FactID: &fact, EvidenceKind: "file", Detail: map[string]any{"p": "x"}, CreatedAt: created},
					{EvidenceID: "e2", DecisionID: "d1", EvidenceKind: "span", CreatedAt: created},
				},
			},
		}
		rec := serve(store, decisionsValidBody)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		var doc struct {
			Count     int              `json:"count"`
			Decisions []map[string]any `json:"decisions"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if doc.Count != 2 || len(doc.Decisions) != 2 {
			t.Fatalf("count/decisions = %d/%d, want 2/2 (%s)", doc.Count, len(doc.Decisions), rec.Body.String())
		}
		first := doc.Decisions[0]
		for _, key := range []string{"decision_id", "decision_type", "repository_id", "source_run_id", "work_item_id", "subject", "confidence_score", "confidence_reason", "provenance_summary", "created_at"} {
			if _, present := first[key]; !present {
				t.Errorf("first row lacks key %q: %v", key, first)
			}
		}
		if first["decision_id"] != "d1" || first["confidence_score"] != 0.9 {
			t.Errorf("first row = %v, want the decision fields", first)
		}
		if first["created_at"] != "2026-03-04T05:06:07Z" {
			t.Errorf("first row created_at = %v, want RFC3339", first["created_at"])
		}
		evidence, ok := first["evidence"].([]any)
		if !ok || len(evidence) != 2 {
			t.Fatalf("first row evidence = %v, want the 2 joined rows", first["evidence"])
		}
		firstEvidence, _ := evidence[0].(map[string]any)
		if firstEvidence["evidence_id"] != "e1" || firstEvidence["fact_id"] != "f1" {
			t.Errorf("first evidence = %v, want e1/f1", firstEvidence)
		}
		secondEvidence, _ := evidence[1].(map[string]any)
		if _, present := secondEvidence["fact_id"]; present {
			t.Errorf("second evidence keeps nil fact_id: %v", secondEvidence)
		}
		// Core emits the evidence key even when the fan-out is empty
		// (see core facts.go: the key is set for every decision when
		// include_evidence rides), so the joined leg pins the empty
		// array, not an absent key.
		evicted, ok := doc.Decisions[1]["evidence"].([]any)
		if !ok || len(evicted) != 0 {
			t.Errorf("second decision evidence = %v, want the empty joined array", doc.Decisions[1]["evidence"])
		}
		if store.evCalls != 2 {
			t.Errorf("evidence calls = %d, want 2 (one fan-out per decision)", store.evCalls)
		}
	})

	t.Run("absent limit defaults to 100", func(t *testing.T) {
		t.Parallel()
		store := &fakeDecisionStore{}
		rec := serve(store, `{"repository_id":"r1","source_run_id":"s1"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		if store.calls != 1 {
			t.Fatalf("store calls = %d, want 1", store.calls)
		}
		if store.got.Limit != 100 {
			t.Errorf("limit = %d, want the core default 100", store.got.Limit)
		}
		if store.got.IncludeEvidence {
			t.Error("include_evidence = true, want the caller shape passed through (absent stays false)")
		}
	})

	t.Run("decision type passes through trimmed pair", func(t *testing.T) {
		t.Parallel()
		store := &fakeDecisionStore{}
		rec := serve(store, `{"repository_id":" r1 ","source_run_id":" s1 ","limit":10}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		if store.got.RepositoryID != "r1" || store.got.SourceRunID != "s1" {
			t.Errorf("filter = %+v, want the trimmed pair", store.got)
		}
	})
}

// TestAdminDecisionsProofRead is the zero-core proof for the
// promoted reader: a P-READ case pairing the shared-key success
// (which must emit >= 1 postgresql/read span from the production
// store path, one for the list plus one for the evidence fan-out)
// with the credential-less denial (absolute span silence) through
// the full production chain with the counting transport. The store
// is the real internal/adminread reader over a fake pool — the fake
// DecisionStore emits no spans, so the success spans can only come
// from the production ListDecisions/ListEvidence. Every leg asserts
// zero core-origin calls.
func TestAdminDecisionsProofRead(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 3, 4, 6, 6, 7, 0, time.UTC)
	store := adminread.NewStoreFromPool(&fakeDecisionsPool{
		decisions: &fakeDecisionRows{staged: true, at: at},
		evidence:  &fakeEvidenceRows{staged: true, at: at},
	})
	srv := NewServer("http://core.invalid", skeletonTestKey).WithDecisionStore(store)
	counter := InstallCoreCallCounter(srv)
	rec := &RecordingStoreSpans{}
	handler := withStoreSpanRecorder(rec, srv.Handler(DefaultV0Sunset))

	RunProof(t, handler, counter, rec, []ProofCase{
		{
			Name: "decisions local success",
			Kind: ProofRead,
			Requests: []ProofRequest{
				{Method: http.MethodPost, Path: "/api/v0/admin/decisions/query", Credential: "valid", Body: decisionsValidBody},
			},
			WantStatuses: []int{http.StatusOK},
			WantSpans: []StoreSpanExpectation{
				{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Min: 2, Max: 2},
			},
		},
		{
			Name: "decisions denial silence",
			Kind: ProofRead,
			Requests: []ProofRequest{
				{Method: http.MethodPost, Path: "/api/v0/admin/decisions/query", Credential: "missing", Body: decisionsValidBody},
			},
			WantStatuses: []int{http.StatusUnauthorized},
			WantSpans:    nil,
		},
	})
}

// decisionsErrorMessage extracts the caller-visible error detail
// from either negotiated shape: the envelope nests it under error,
// the dest plain shape carries it as message, and the apilib plain
// shape carries it as detail (the required-field 400s render through
// the pinned apilib writer). It decodes the JSON so HTML-escaped
// bytes compare canonical.
func decisionsErrorMessage(t *testing.T, body []byte) string {
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
	if detail, ok := doc["detail"].(string); ok {
		return detail
	}
	t.Fatalf("error body carries no message: %v", doc)
	return ""
}

// parseDecisionsTime parses one RFC3339 instant for the parity
// fixtures. The apilib writer renders timestamps in RFC3339, so the
// fixtures pin the same rendering the success-shape case asserts.
func parseDecisionsTime(t *testing.T, value string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("bad fixture instant %q: %v", value, err)
	}
	return at
}
