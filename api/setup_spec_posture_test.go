// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSetupSpecPosture_StaysCoreSide pins the eshu-api#44 outcome as
// revisited by eshu-hq/eshu-api#164: the setup OpenAPI fragment rides
// the served document with its core public-route markers intact. The
// dest-served GET /api/v0/openapi.json now serves pre-auth like core
// (the anticipated posture revisit: core's spec route is public and
// its self-entry carries no marker, so the served document stays
// truthful with no new marker); the four C-c1 carve-out routes stay
// public alongside it. The marker rule below still holds: markers only
// on the public setup and login-picker entries, never on an authed
// route — and the spec self-entry stays unmarked exactly like core's.
func TestSetupSpecPosture_StaysCoreSide(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"needs_setup":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	// The four carve-out routes stay public: no credential, no dest
	// 401 — the request reaches the core.
	for _, spelling := range setupSpellings() {
		var body io.Reader
		if spelling.method != http.MethodGet {
			body = strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`)
		}
		req := httptest.NewRequest(spelling.method, spelling.path, body)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized {
			t.Errorf("%s %q: status = 401, want public (forwarded, not gated)", spelling.method, spelling.path)
		}
	}
	if core.hits != len(setupSpellings()) {
		t.Errorf("upstream hits = %d, want %d (every setup spelling forwards)", core.hits, len(setupSpellings()))
	}

	// The served spec is public like core's: the same unauthenticated
	// caller that reaches the setup routes gets 200 on the spec.
	req := httptest.NewRequest(http.MethodGet, "/api/v0/openapi.json", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("unauthenticated spec status = %d, want 200 (spec serves pre-auth)", rec.Code)
	}

	// The authed served spec carries no setup path keys and no
	// "security": [] public-route markers: dest fragments document
	// posture in prose, never with a public-security override inside
	// an authed document.
	authed := authedRequest(http.MethodGet, "/api/v0/openapi.json")
	authedRec := httptest.NewRecorder()
	handler.ServeHTTP(authedRec, authed)
	if authedRec.Code != http.StatusOK {
		t.Fatalf("authenticated spec status = %d, want 200", authedRec.Code)
	}
	raw, _ := io.ReadAll(authedRec.Result().Body)
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("served spec is not valid JSON: %v", err)
	}
	paths, _ := doc["paths"].(map[string]any)
	for _, path := range []string{
		"/api/v0/auth/setup-state",
		"/api/v0/auth/setup/claim",
		"/api/v0/auth/setup/admin",
		"/api/v0/auth/setup/mfa",
	} {
		if _, ok := paths[path]; !ok {
			t.Errorf("served spec lacks %s, want the full-surface setup keys served", path)
		}
	}
	// Marker rule (per-entry, truthful): the setup wizard routes and
	// the login-picker GET are public pre-auth in dest (the C-c1
	// carve-out plus the permanent picker), so their entries carry
	// core's `"security": []` public-route markers intact — stripping a
	// marker would make the served spec lie about the route's posture.
	// No authed route may carry a marker. Match the marker key with an
	// empty-array value (`"security":[]` in the re-marshaled form), not
	// the bare word — the infra fragment ports a resource_kind enum
	// value named "security" that is not a public-route posture marker.
	marked := map[string]bool{}
	for path, item := range paths {
		rawItem, _ := json.Marshal(item)
		if !strings.Contains(string(rawItem), `"security":[]`) {
			continue
		}
		marked[path] = true
	}
	for _, path := range []string{
		"/api/v0/auth/providers",
		"/api/v0/auth/setup-state",
		"/api/v0/auth/setup/claim",
		"/api/v0/auth/setup/admin",
		"/api/v0/auth/setup/mfa",
	} {
		if !marked[path] {
			t.Errorf("served spec entry %s carries no security marker, want core's truthful public-route marker intact (stripping it would lie about the route's posture)", path)
			continue
		}
		delete(marked, path)
	}
	for path := range marked {
		t.Errorf("served spec entry %s carries a security marker, want markers only on the public setup and login-picker entries", path)
	}
}
