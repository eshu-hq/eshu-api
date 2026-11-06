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

// D2 Leaf B served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, part of #6707). The openapi package tests pin the
// assembled document; these tests pin that the native openapi.json
// route serves it to keyed callers with the wave path keys present —
// for both a configured and an unconfigured core origin — while the
// key gate still fronts the route (the standing no-new-public-surface
// decision for the assembled spec).

// leafbWaveSpecKeys are the path keys the D2 wave adds to the served
// document: the openapi.json self entry plus the twelve data-route
// keys (the provider-config create/update POSTs share the list/detail
// keys, so thirteen served keys cover the wave's data routes).
var leafbWaveSpecKeys = []string{
	"/api/v0/openapi.json",
	"/api/v0/capabilities",
	"/api/v0/answer-quality/score",
	"/api/v0/investigation/packet/render",
	"/api/v0/report/bundle/capture",
	"/api/v0/report/bundle/validate",
	"/api/v0/service/report/compose",
	"/api/v0/ask",
	"/api/v0/services/{service_name}/intelligence-report",
	"/api/v0/auth/providers",
	"/api/v0/auth/admin/provider-configs",
	"/api/v0/auth/admin/provider-configs/{provider_config_id}",
	"/api/v0/auth/admin/provider-configs/{provider_config_id}/revisions",
}

// TestLeafBNative_ServesWaveSpecKeys pins that the native spec route
// serves the wave keys: authed callers get the dest-assembled document
// with every wave path, in both negotiated shapes, with or without a
// core origin configured.
func TestLeafBNative_ServesWaveSpecKeys(t *testing.T) {
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
		for _, path := range leafbWaveSpecKeys {
			if _, ok := paths[path]; !ok {
				t.Errorf("origin %q: served spec lacks wave key %s", origin, path)
			}
		}
		if item, ok := paths["/api/v0/ask"].(map[string]any); ok {
			if op, _ := item["post"].(map[string]any); op["operationId"] != "ask" {
				t.Errorf("origin %q: ask operationId = %v, want ask", origin, op["operationId"])
			}
		}
		if item, ok := paths["/api/v0/capabilities"].(map[string]any); ok {
			if op, _ := item["get"].(map[string]any); op["operationId"] != "listCapabilities" {
				t.Errorf("origin %q: capabilities operationId = %v, want listCapabilities", origin, op["operationId"])
			}
		}
	}
}
