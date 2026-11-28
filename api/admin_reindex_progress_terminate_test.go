// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"

	"github.com/eshu-hq/eshu-api/internal/adminread"
)

// Admin reindex-progress native termination (satellite
// eshu-hq/eshu-api#170, the third D3 store-backed promotion, lifting
// the last Gap 02 trio GET out of the D3 wave). TDD record: this
// suite failed RED before the lane landed (the route constant, the
// terminated handler, the store field, and the reader store methods
// did not exist, so every assertion below failed at compile; the
// shared-key legs rode the generic D3 gate, so the zero-core
// assertions failed) and is green with it. This file owns the
// termination half: the freeze, the local termination, the progress
// shape parity, the apilib matrix, and the zero-core P-READ proof.
// The v1 alias, census, deny branch, fail-closed origin, spec key,
// benchmark, and gate matrix live in
// api/admin_reindex_progress_parity_test.go.
//
// Lane rule (arbiter D-wave ruling, decision 4, via the D3 wave): the
// shared-key class terminates locally on the pinned apilib
// ServeReindexProgress body served from the reader-role PG store
// with zero core contact; credential classes dest cannot resolve
// (scoped tokens, OIDC bearers, browser sessions) relay explicitly on
// the untouched D3 gate so core judges them; credential-less and
// junk-cookie callers keep the standing 401 with zero core contact.
// A missing store answers the apilib local 503 (never a forward); a
// failed store answers the fixed 500 (never the backend text). The
// only dest-synthesized errors on the terminated path are the gate
// 401, the family 403, the apilib 400s, and the permanent local 503
// when no core origin is configured (never Retry-After). A store
// error answers the fixed "reindex progress query failed" detail,
// never the backend text.
//
// Progress validation (core parity, preserved dest-side by the
// pinned apilib v0.3.0): scope_id is required (1-100, none blank,
// trimmed, deduplicated, sorted) and fleet_requested_at must be
// RFC 3339 when present — the apilib renders the core-identical
// details, this lane owns only the gate, the store handle, and the
// telemetry.
//
// Proof vocabulary: P-READ carries the RunProof case — success emits
// >= 1 postgresql/read span from the production reader store
// (internal/adminread over a fake pool) through the context recorder
// with zero core-origin calls; the denial leg asserts absolute span
// silence.

// TestAdminReindexProgressRouteFrozen pins the promoted surface: the
// single reindex-progress GET the lane terminates. A route rename or
// rehome fails here until the ownership map, the spec fragment, and
// this mirror move together.
func TestAdminReindexProgressRouteFrozen(t *testing.T) {
	t.Parallel()

	if adminReindexProgressRoute != "GET /api/v0/admin/reindex/progress" {
		t.Fatalf("adminReindexProgressRoute = %q, want the frozen reindex-progress GET", adminReindexProgressRoute)
	}
}

// TestAdminReindexProgressTerminatesLocally pins that the promoted
// route is an explicit registration ahead of the generic proxy: a
// shared-key call serves the apilib progress body on the native lane
// with zero core contact — exact path and trailing-slash twin
// alike.
func TestAdminReindexProgressTerminatesLocally(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	store := &fakeMaintenanceStore{
		fleet: at,
		states: map[string]apilibadmin.FullReconcileState{
			"s1": {HasProjectedFull: true, LastStoredFullAt: at, ActiveGenerationID: "g7"},
		},
	}

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel).WithReindexProgress(store)
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/admin/reindex/progress?scope_id=s1",
		"/api/v0/admin/reindex/progress/?scope_id=s1",
	} {
		req := authedRequest(http.MethodGet, target)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want the terminated 200", target, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Errorf("GET %s: Content-Type = %q, want the JSON document shape", target, got)
		}
		var doc struct {
			Status string `json:"status"`
			Scopes []struct {
				ScopeID string `json:"scope_id"`
				State   string `json:"state"`
			} `json:"scopes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("GET %s: body is not JSON: %v", target, rec.Body.String())
		}
		if doc.Status != "complete" || len(doc.Scopes) != 1 || doc.Scopes[0].ScopeID != "s1" || doc.Scopes[0].State != "satisfied" {
			t.Errorf("GET %s: body = %s, want the one satisfied scope", target, rec.Body.String())
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (shared-key terminates locally)", core.hits)
	}
	counter.AssertZero(t, "shared-key local termination")
	if store.calls != 6 {
		t.Errorf("store calls = %d, want 6 (3 reads each on exact path plus twin)", store.calls)
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

// TestAdminReindexProgressShapeParity pins the core-identical
// progress shapes through the terminated lane: the empty read (no
// watermark anywhere) answers the incomplete report with the null
// fleet instant and the never-seen scope, while an in-progress read
// keeps the scope first and the watermarks exact.
func TestAdminReindexProgressShapeParity(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	serve := func(store *fakeMaintenanceStore, target string) *httptest.ResponseRecorder {
		srv := NewServer(upstream.URL, skeletonTestKey).WithReindexProgress(store)
		rec := httptest.NewRecorder()
		srv.Handler(DefaultV0Sunset).ServeHTTP(rec, authedRequest(http.MethodGet, target))
		return rec
	}

	t.Run("empty read is incomplete with null fleet", func(t *testing.T) {
		t.Parallel()
		rec := serve(&fakeMaintenanceStore{states: map[string]apilibadmin.FullReconcileState{}}, "/api/v0/admin/reindex/progress?scope_id=s1")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		var doc struct {
			Status         string           `json:"status"`
			FleetRequested *string          `json:"fleet_requested_at"`
			Counts         map[string]int   `json:"counts"`
			Scopes         []map[string]any `json:"scopes"`
			Truncated      bool             `json:"truncated"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("body is not JSON: %v", rec.Body.String())
		}
		if doc.Status != "incomplete" {
			t.Errorf("status = %q, want incomplete (never-seen scope)", doc.Status)
		}
		if doc.FleetRequested != nil {
			t.Errorf("fleet_requested_at = %v, want null (no watermark anywhere)", doc.FleetRequested)
		}
		if doc.Counts["never_seen"] != 1 {
			t.Errorf("counts = %v, want one never-seen scope", doc.Counts)
		}
		if len(doc.Scopes) != 1 || doc.Scopes[0]["state"] != "never_seen" || doc.Scopes[0]["scope_id"] != "s1" {
			t.Errorf("scopes = %v, want the one never-seen scope", doc.Scopes)
		}
		if doc.Truncated {
			t.Errorf("truncated = true, want false (one scope fits the samples)")
		}
	})

	t.Run("in-progress read keeps watermarks exact", func(t *testing.T) {
		t.Parallel()
		fleet := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
		stored := fleet.Add(-time.Hour)
		rec := serve(&fakeMaintenanceStore{
			fleet: fleet,
			states: map[string]apilibadmin.FullReconcileState{
				"s1": {HasProjectedFull: true, LastStoredFullAt: stored, ActiveGenerationID: "g7"},
			},
		}, "/api/v0/admin/reindex/progress?scope_id=s1")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		var doc struct {
			Status string         `json:"status"`
			Fleet  string         `json:"fleet_requested_at"`
			Counts map[string]int `json:"counts"`
			Scopes []struct {
				ScopeID   string `json:"scope_id"`
				State     string `json:"state"`
				Requested string `json:"requested_at"`
				Stored    string `json:"last_stored_full_at"`
				Active    string `json:"active_generation_id"`
			} `json:"scopes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("body is not JSON: %v", rec.Body.String())
		}
		if doc.Status != "incomplete" {
			t.Errorf("status = %q, want incomplete (stored full predates the fleet)", doc.Status)
		}
		if doc.Fleet != "2026-03-04T05:06:07Z" {
			t.Errorf("fleet_requested_at = %q, want the fleet instant", doc.Fleet)
		}
		if len(doc.Scopes) != 1 || doc.Scopes[0].State != "pending" {
			t.Errorf("scopes = %+v, want the one pending scope", doc.Scopes)
		}
		first := doc.Scopes[0]
		if first.Requested != "2026-03-04T05:06:07Z" || first.Stored != "2026-03-04T04:06:07Z" || first.Active != "g7" {
			t.Errorf("scope watermarks = %+v, want the compared pair plus g7", first)
		}
	})

	t.Run("missing scope_id is 400 with zero store contact", func(t *testing.T) {
		t.Parallel()
		store := &fakeMaintenanceStore{}
		rec := serve(store, "/api/v0/admin/reindex/progress")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if store.calls != 0 {
			t.Errorf("store calls = %d, want 0 (validation fails before the store)", store.calls)
		}
		if core.hits != 0 {
			t.Errorf("upstream hits = %d, want 0 (validation never forwards)", core.hits)
		}
	})

	t.Run("malformed fleet override is 400", func(t *testing.T) {
		t.Parallel()
		rec := serve(&fakeMaintenanceStore{}, "/api/v0/admin/reindex/progress?scope_id=s1&fleet_requested_at=yesterday")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing store is local 503 never a forward", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(upstream.URL, skeletonTestKey)
		rec := httptest.NewRecorder()
		srv.Handler(DefaultV0Sunset).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v0/admin/reindex/progress?scope_id=s1"))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		var doc map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("body is not JSON: %v", rec.Body.String())
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
		rec := serve(&fakeMaintenanceStore{err: errors.New("sql: connection refused host=db-internal password=hunter2")}, "/api/v0/admin/reindex/progress?scope_id=s1")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500 (%s)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "reindex progress query failed") {
			t.Errorf("body = %s, want the fixed failure detail", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "hunter2") {
			t.Errorf("backend text leaked into body: %s", rec.Body.String())
		}
	})
}

// TestAdminReindexProgressProofRead is the zero-core proof for the
// promoted reader: a P-READ case pairing the shared-key success
// (which must emit >= 1 postgresql/read span from the production
// store path) with the credential-less denial (absolute span
// silence) through the full production chain with the counting
// transport. The store is the real internal/adminread reader over a
// fake pool — the fake MaintenanceStore emits no spans, so the
// success spans can only come from the production maintenance
// reads. Every leg asserts zero core-origin calls.
func TestAdminReindexProgressProofRead(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	store := adminread.NewStoreFromPool(&fakeProgressPool{at: at})
	srv := NewServer("http://core.invalid", skeletonTestKey).WithReindexProgress(store)
	counter := InstallCoreCallCounter(srv)
	rec := &RecordingStoreSpans{}
	handler := withStoreSpanRecorder(rec, srv.Handler(DefaultV0Sunset))

	RunProof(t, handler, counter, rec, []ProofCase{
		{
			Name: "reindex-progress local success",
			Kind: ProofRead,
			Requests: []ProofRequest{
				{Method: http.MethodGet, Path: "/api/v0/admin/reindex/progress?scope_id=s1", Credential: "valid"},
			},
			WantStatuses: []int{http.StatusOK},
			WantSpans: []StoreSpanExpectation{
				{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Min: 3, Max: 3},
			},
		},
		{
			Name: "reindex-progress denial silence",
			Kind: ProofRead,
			Requests: []ProofRequest{
				{Method: http.MethodGet, Path: "/api/v0/admin/reindex/progress?scope_id=s1", Credential: "missing"},
			},
			WantStatuses: []int{http.StatusUnauthorized},
			WantSpans:    nil,
		},
	})
}
