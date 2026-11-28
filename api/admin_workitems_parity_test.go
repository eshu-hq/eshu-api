// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

// Admin work-items query bounded-list parity (satellite
// eshu-hq/eshu-api#335). This file owns the local matrix half of the
// lane proof: the 400/500/503 legs against the #456 consumer values,
// the v1 alias, the census, the deny branch, the fail-closed origin,
// the spec key, and the dest-side benchmark. The zero-core P-READ
// proof and the gate matrix live in
// api/admin_workitems_terminate_test.go.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// TestAdminWorkItemsV1AliasServesLocal pins that the v1 alias rewrite
// lands the promoted route on the same local handler: the shared-key
// v1 call serves the store body with zero core contact.
func TestAdminWorkItemsV1AliasServesLocal(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey).WithWorkItemStore(&fakeWorkItemStore{items: []apilibadmin.WorkItem{
		{WorkItemID: "w9", ScopeID: "s1", GenerationID: "g1", Stage: "build", Domain: "d", Status: "pending"},
	}})
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	req := workItemsAuthedPOST(t, "/api/v1/admin/work-items/query", `{"limit":10}`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"w9"`) {
		t.Errorf("body = %s, want the store row", rec.Body.String())
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (v1 alias serves locally)", core.hits)
	}
	counter.AssertZero(t, "v1 alias local termination")
}

// TestAdminWorkItemsCensus is the static-analysis half of the
// zero-core claim: the shared-key handler closure reaches no core
// sink. The foreign classes relay by design (pinned above), so the
// RouteClaims cover only the proven shared-key class (decision 4).
func TestAdminWorkItemsCensus(t *testing.T) {
	t.Parallel()

	AssertNoCorePath(t, []RouteClaim{
		{Pattern: "POST /api/v0/admin/work-items/query", Handler: "Server.serveAdminWorkItemsTerminated", Credential: "shared-key"},
	})
}

// TestAdminWorkItemsDenyBranchShapePinned pins the deny shape for the
// day dest gains a scoped/session resolver: a non-AllScopes identity
// is denied with the documented family 403 in both negotiated shapes
// and the core is never contacted. The branch is unreachable on the
// live route today (the wave gate passes true); this pins the
// contract, not a live path.
func TestAdminWorkItemsDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey).WithWorkItemStore(&fakeWorkItemStore{})

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := workItemsAuthedPOST(t, "/api/v0/admin/work-items/query", `{"limit":10}`)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		srv.serveAdminWorkItemsTerminatedAs(rec, req, false)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Accept %q: status = %d, want 403", accept, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), d3FamilyDenyMessage) {
			t.Errorf("Accept %q: body = %q, want the family-403 message", accept, body)
		}
	}
}

// TestAdminWorkItemsUnconfiguredOriginFailsClosed pins the fail-closed
// origin on the terminated path: without a valid core origin every
// shared-key call answers the permanent local 503 (no Retry-After) in
// both wire shapes, with zero store contact.
func TestAdminWorkItemsUnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"", "://missing-scheme-host"} {
		store := &fakeWorkItemStore{}
		srv := NewServer(origin, skeletonTestKey).WithWorkItemStore(store)
		handler := srv.Handler(DefaultV0Sunset)

		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := workItemsAuthedPOST(t, "/api/v0/admin/work-items/query", `{"limit":10}`)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("origin %q Accept %q: status = %d, want 503", origin, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("origin %q Accept %q: Retry-After is set on the permanent 503", origin, accept)
			}
		}
		if store.calls != 0 {
			t.Errorf("origin %q: store calls = %d, want 0 (fail closed before the store)", origin, store.calls)
		}
	}
}

// TestAdminWorkItemsSpecKeyServed pins that the promotion keeps the
// route's served-spec entry: keyed callers get the dest-assembled
// document with the work-items query path key in both negotiated
// shapes, with or without a core origin configured.
func TestAdminWorkItemsSpecKeyServed(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"http://core.invalid", ""} {
		srv := NewServer(origin, skeletonTestKey).WithWorkItemStore(&fakeWorkItemStore{})
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodGet, "/api/v0/openapi.json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("origin %q: status = %d, want 200", origin, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("origin %q: served spec is not valid JSON: %v", origin, err)
		}
		paths, _ := doc["paths"].(map[string]any)
		item, ok := paths["/api/v0/admin/work-items/query"].(map[string]any)
		if !ok {
			t.Errorf("origin %q: served spec lacks the work-items query key", origin)
			continue
		}
		if _, ok := item["post"].(map[string]any); !ok {
			t.Errorf("origin %q: served spec lacks post under the work-items query key", origin)
		}
	}
}

// BenchmarkAdminWorkItemsTerminated measures the promoted path for a
// shared-key query POST through the full production chain. The stub
// core answers from memory but the lane never dials it, and the fake
// store answers from memory too — the numbers bound dest-side cost
// only (bounded pool arithmetic, no core).
func BenchmarkAdminWorkItemsTerminated(b *testing.B) {
	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	srv := NewServer(upstream.URL, skeletonTestKey).WithWorkItemStore(&fakeWorkItemStore{})
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	warmReq := authedRequest(http.MethodPost, "/api/v0/admin/work-items/query")
	warmReq.Body = io.NopCloser(strings.NewReader(`{"limit":10}`))
	warmRec := httptest.NewRecorder()
	handler.ServeHTTP(warmRec, warmReq)
	if warmRec.Code != http.StatusOK {
		b.Fatalf("warmup status = %d, want 200", warmRec.Code)
	}
	if got := counter.Calls(); got != 0 {
		b.Fatalf("warmup core-origin calls = %d, want 0 (no core)", got)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := authedRequest(http.MethodPost, "/api/v0/admin/work-items/query")
		req.Body = io.NopCloser(strings.NewReader(`{"limit":10}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
	if got := counter.Calls(); got != 0 {
		b.Fatalf("core-origin calls = %d, want 0 (no core)", got)
	}
}

// parseWorkItemsTime parses one RFC3339 instant for the parity
// fixtures. The apilib writer renders timestamps in RFC3339, so the
// fixtures pin the same rendering the success-shape case asserts.
func parseWorkItemsTime(t *testing.T, value string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("bad fixture instant %q: %v", value, err)
	}
	return at
}
