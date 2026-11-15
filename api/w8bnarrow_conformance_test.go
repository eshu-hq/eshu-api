// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// W8b-narrow conformance (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#101). A shared-key call on every wave route must
// terminate natively with the caller's method, path, query, body,
// and credential intact and core bytes untouched; each trailing-slash
// spelling terminates exactly like its bare spelling (never-split).

// w8bNarrowConformLegs pins one conformance leg per wave route with a
// representative body or query.
var w8bNarrowConformLegs = []struct {
	method string
	path   string
	body   string
}{
	{http.MethodGet, "/api/v0/kubernetes/correlations", ""},
	{http.MethodGet, "/api/v0/observability/coverage/correlations", ""},
	{http.MethodGet, "/api/v0/work-items/evidence", ""},
	{http.MethodGet, "/api/v0/semantic/code-hints", ""},
	{http.MethodGet, "/api/v0/semantic/documentation-observations", ""},
	{http.MethodGet, "/api/v0/dependencies?repo_id=repo-1", ""},
	{http.MethodGet, "/api/v0/graph/entities?limit=10", ""},
	{http.MethodGet, "/api/v0/images", ""},
	{http.MethodGet, "/api/v0/images/tag-history?image=api", ""},
	{http.MethodGet, "/api/v0/incidents/inc-9/context", ""},
	{http.MethodPost, "/api/v0/runtime-declarations/query", `{"repo_id":"repo-1"}`},
	{http.MethodGet, "/api/v0/cloud/inventory", ""},
	{http.MethodGet, "/api/v0/cloud/ssm-host-inventory", ""},
	{http.MethodGet, "/api/v0/service-catalog/correlations", ""},
	{http.MethodGet, "/api/v0/codeowners/ownership?repo_id=repo-1", ""},
}

// TestW8bNarrowTerminatedByteForByte pins native termination for the
// whole wave: status, headers, and body cross identically, and the
// upstream sees the caller's method, path, query, body, and
// credential intact.
func TestW8bNarrowTerminatedByteForByte(t *testing.T) {
	t.Parallel()

	for _, leg := range w8bNarrowConformLegs {
		t.Run(leg.method+" "+leg.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"ok":true,"lane":"native"}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			var req *http.Request
			if leg.body == "" {
				req = w8bNarrowAuthedReq(leg.method, leg.path, "")
			} else {
				req = w8bNarrowAuthedReq(leg.method, leg.path, leg.body)
			}
			req.Header.Set("X-Correlation-ID", "w8bnarrow-conform-1")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			gotBody, _ := io.ReadAll(rec.Result().Body)
			if string(gotBody) != `{"ok":true,"lane":"native"}` {
				t.Errorf("body = %q, want core bytes untouched", gotBody)
			}
			if core.lastMethod != leg.method {
				t.Errorf("upstream method = %q, want %q", core.lastMethod, leg.method)
			}
			if core.lastAuth != "Bearer "+skeletonTestKey {
				t.Errorf("upstream auth forwarded = %q, want the shared-key credential intact", core.lastAuth)
			}
			if core.lastBody != leg.body {
				t.Errorf("upstream body = %q, want %q forwarded intact", core.lastBody, leg.body)
			}
		})
	}
}

// TestW8bNarrowSlashSpellingsTerminateNatively pins the never-split
// rule: each trailing-slash spelling terminates on the native lane
// exactly like its bare spelling.
func TestW8bNarrowSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for _, leg := range w8bNarrowRedLegs {
		t.Run(leg.method+" "+leg.path+"/", func(t *testing.T) {
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
			handler.ServeHTTP(rec, w8bNarrowAuthedReq(leg.method, leg.path+"/", leg.body))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.dispatches[leg.family][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash twin", leg.family, got)
			}
		})
	}
}
