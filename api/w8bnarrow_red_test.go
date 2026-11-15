// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// W8b-narrow ahead-of-relay RED (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#101). The 15 narrow-family routes must terminate
// on the native lane for a shared-key caller. Until the lane lands,
// the calls cross the generic relay and this test is RED.
var w8bNarrowRedLegs = []struct {
	method string
	path   string
	body   string
	family string
}{
	{http.MethodGet, "/api/v0/kubernetes/correlations", "", "kubernetes"},
	{http.MethodGet, "/api/v0/observability/coverage/correlations", "", "observability"},
	{http.MethodGet, "/api/v0/work-items/evidence", "", "work_items"},
	{http.MethodGet, "/api/v0/semantic/code-hints", "", "semantic"},
	{http.MethodGet, "/api/v0/semantic/documentation-observations", "", "semantic"},
	{http.MethodGet, "/api/v0/dependencies", "", "dependencies"},
	{http.MethodGet, "/api/v0/graph/entities", "", "graph_entities"},
	{http.MethodGet, "/api/v0/images", "", "images"},
	{http.MethodGet, "/api/v0/images/tag-history", "", "images"},
	{http.MethodGet, "/api/v0/incidents/abc123/context", "", "incidents"},
	{http.MethodPost, "/api/v0/runtime-declarations/query", `{}`, "runtime_declarations"},
	{http.MethodGet, "/api/v0/cloud/inventory", "", "cloud_inventory"},
	{http.MethodGet, "/api/v0/cloud/ssm-host-inventory", "", "cloud_inventory"},
	{http.MethodGet, "/api/v0/service-catalog/correlations", "", "service_catalog"},
	{http.MethodGet, "/api/v0/codeowners/ownership", "", "codeowners"},
}

func w8bNarrowAuthedReq(method, path, body string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	req.Header.Set("Accept", "application/json")
	return req
}

func TestW8bNarrowWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, leg := range w8bNarrowRedLegs {
		t.Run(leg.method+" "+leg.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, w8bNarrowAuthedReq(leg.method, leg.path, leg.body))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			if core.lastPath != leg.path {
				t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, leg.path)
			}
			snap := tel.snapshot()
			lanes, ok := snap.dispatches[leg.family]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", leg.family)
			}
			if lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", leg.family, lanes)
			}
			if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1", leg.family, got)
			}
		})
	}
}
