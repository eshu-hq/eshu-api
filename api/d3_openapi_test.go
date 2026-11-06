// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// D3 served-spec proof at the route level (Refs eshu-hq/eshu#9669,
// part of #6707). The openapi package tests pin the assembled
// document; these tests pin that the native openapi.json route serves
// it to keyed callers with the D3 path keys present — for both a
// configured and an unconfigured core origin — while the key gate
// still fronts the route (the standing no-new-public-surface decision
// for the assembled spec).

// d3SpecKeys are the path keys the D3 wave adds to the served
// document: the six admin query POSTs plus the invitations list GET
// (whose create POST shares the key and stays relay until D8, so the
// served entry is get-only).
var d3SpecKeys = []string{
	"/api/v0/admin/work-items/query",
	"/api/v0/admin/decisions/query",
	"/api/v0/admin/dead-letters/query",
	"/api/v0/admin/input-invalid-facts/query",
	"/api/v0/admin/changed-since/poisoned-links/query",
	"/api/v0/admin/replay-events/query",
	"/api/v0/auth/local/invitations",
}

// TestD3Native_ServesWaveSpecKeys pins that the native spec route
// serves the D3 keys: authed callers get the dest-assembled document
// with every D3 path, in both negotiated shapes, with or without a
// core origin configured.
func TestD3Native_ServesWaveSpecKeys(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"http://core.invalid", ""} {
		srv := NewServer(origin, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodGet, "/api/v0/openapi.json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("origin %q: status = %d, want 200", origin, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Errorf("origin %q: Content-Type = %q, want the JSON document shape", origin, got)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("origin %q: served spec is not valid JSON: %v", origin, err)
		}
		paths, _ := doc["paths"].(map[string]any)
		for _, path := range d3SpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks D3 key %s", origin, path)
			}
		}
		if item, ok := paths["/api/v0/auth/local/invitations"].(map[string]any); ok {
			if op, _ := item["get"].(map[string]any); op["operationId"] != "listAdminInvitations" {
				t.Errorf("origin %q: invitations operationId = %v, want listAdminInvitations", origin, op["operationId"])
			}
			if _, hasPost := item["post"]; hasPost {
				t.Errorf("origin %q: invitations entry carries post, want get-only until D8", origin)
			}
		}
	}
}
