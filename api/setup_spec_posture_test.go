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

// TestSetupSpecPosture_StaysCoreSide pins the eshu-api#44 outcome: the
// setup OpenAPI fragment stays core-side. The dest-served
// GET /api/v0/openapi.json stays inside RequireAuth (like every other
// dest /api/* route), so it serves no setup path keys; the four C-c1
// carve-out routes stay public alongside it. A future lane that ports
// the core setup fragment (whose operations all carry "security": [])
// must revisit this posture instead of landing silently: embedding
// public-route docs in the authed spec would misrepresent posture, and
// opening a public spec route would widen the surface beyond the four
// arbiter-approved routes.
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

	// The served spec stays behind the key gate: the same
	// unauthenticated caller that reaches the setup routes gets 401
	// on the spec.
	req := httptest.NewRequest(http.MethodGet, "/api/v0/openapi.json", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated spec status = %d, want 401 (spec stays authed)", rec.Code)
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
		if _, ok := paths[path]; ok {
			t.Errorf("served spec contains %s, want core-side only (eshu-api#44)", path)
		}
	}
	// D2 Leaf B revisit (recorded, not silent): the auth-providers
	// entry truthfully carries core's `"security": []` public-route
	// marker — the login-picker GET is permanently public pre-auth and
	// this lane terminates it publicly, so stripping the marker would
	// make the served spec lie about the route's posture. That differs
	// from the setup fragment (#44): the wizard is a temporary
	// first-run flow whose docs stay core-side entirely. The posture
	// rule is therefore per-entry now: no setup path keys, and a
	// security marker only on the permanently-public login-picker
	// entry. The next lane that ports another public-marked fragment
	// revisits this test again.
	// D4 Native-next revisit (recorded, not silent): match the marker
	// key with an empty-array value (`"security":[]` in the re-marshaled
	// form), not the bare word — the infra fragment ports a
	// resource_kind enum value named "security" that is not a
	// public-route posture marker.
	providersMarked := false
	for path, item := range paths {
		rawItem, _ := json.Marshal(item)
		if !strings.Contains(string(rawItem), `"security":[]`) {
			continue
		}
		if path != "/api/v0/auth/providers" {
			t.Errorf("served spec entry %s carries a security marker, want markers only on the public login-picker entry", path)
			continue
		}
		providersMarked = true
	}
	if !providersMarked {
		t.Errorf("served spec entry /api/v0/auth/providers carries no security marker, want core's truthful public-route marker intact (stripping it would lie about the route's posture)")
	}
}
