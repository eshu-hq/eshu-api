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

// TestOpenAPINative_UnauthenticatedIs401 pins that the native spec route
// keeps the dest 401 shape: the key gate still fronts /api/*, envelope
// callers get the envelope error, everyone else the plain error_code body.
func TestOpenAPINative_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v0/openapi.json", nil)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Accept %q: status = %d, want 401", accept, rec.Code)
		}
		if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Errorf("Accept %q: WWW-Authenticate = %q, want bare Bearer", got, accept)
		}
	}
}

// TestOpenAPINative_ServesSignInPolicyFamily pins the dest-served
// openapi.json for the sign-in-policy family (satellite-design Decision
// 5(c)): the native route answers 200 with the family's paths and pinned
// operationIds, and it answers from compiled constants — no core fetch, so
// an unconfigured origin still serves the spec.
func TestOpenAPINative_ServesSignInPolicyFamily(t *testing.T) {
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
		for path, want := range map[string]map[string]string{
			"/api/v0/auth/sign-in-policy":       {"get": "getPublicSignInPolicy"},
			"/api/v0/auth/admin/sign-in-policy": {"get": "getAdminSignInPolicy", "patch": "updateAdminSignInPolicy"},
		} {
			item, ok := paths[path].(map[string]any)
			if !ok {
				t.Errorf("origin %q: served spec lacks %s", origin, path)
				continue
			}
			for operation, wantID := range want {
				op, _ := item[operation].(map[string]any)
				if got := op["operationId"]; got != wantID {
					t.Errorf("origin %q: %s %s operationId = %v, want %s", origin, path, operation, got, wantID)
				}
			}
		}
	}
}

// TestOpenAPINative_ServesTerminalFailureFamily pins the dest-served
// openapi.json for the terminal-failure family (satellite-design Decision
// 5(c)): the native route answers 200 with the family's paths, and it
// answers from compiled constants — no core fetch, so an unconfigured
// origin still serves the spec. The pinned fragment carries no
// operationId entries, so the paths are pinned by summary here; the
// lossless-assembly Spec tests carry the operationId-preservation half
// (there are none to invent).
func TestOpenAPINative_ServesTerminalFailureFamily(t *testing.T) {
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
		for path, wantSummary := range map[string]string{
			"/api/v0/admin/collector-failures/acknowledge":            "Acknowledge a collector terminal failure",
			"/api/v0/admin/collector-failures/acknowledgements/query": "List collector terminal failure acknowledgements",
			"/api/v0/admin/collector-failures/query":                  "List the failed scopes of one collector instance",
		} {
			item, ok := paths[path].(map[string]any)
			if !ok {
				t.Errorf("origin %q: served spec lacks %s", origin, path)
				continue
			}
			op, _ := item["post"].(map[string]any)
			if got := op["summary"]; got != wantSummary {
				t.Errorf("origin %q: %s post summary = %v, want %q", origin, path, got, wantSummary)
			}
		}
	}
}

// TestOpenAPINative_ServesRetirementFamily pins the dest-served
// openapi.json for the retirement family (satellite-design Decision
// 5(c)): the native route answers 200 with the family's path, and it
// answers from compiled constants — no core fetch, so an unconfigured
// origin still serves the spec.
func TestOpenAPINative_ServesRetirementFamily(t *testing.T) {
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
		item, ok := paths["/api/v0/admin/repository-retirements"].(map[string]any)
		if !ok {
			t.Errorf("origin %q: served spec lacks /api/v0/admin/repository-retirements", origin)
			continue
		}
		post, _ := item["post"].(map[string]any)
		if got := post["summary"]; got != "Retire repositories: mark, fence, then purge their indexed state" {
			t.Errorf("origin %q: retirement post summary = %v, want the pinned summary", origin, got)
		}
	}
}

// TestOpenAPINative_ServesSuppressionFamily pins the dest-served
// openapi.json for the suppression family (satellite-design Decision
// 5(c), D0 backfill): the native route answers 200 with the suppression
// upsert path and pinned operationId, and it answers from compiled
// constants — no core fetch, so an unconfigured origin still serves the
// spec. The route itself stays on its existing tier-C write-through
// dispatch; this lane ports the fragment only.
func TestOpenAPINative_ServesSuppressionFamily(t *testing.T) {
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
		body, _ := io.ReadAll(rec.Result().Body)
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("origin %q: served spec is not valid JSON: %v", origin, err)
		}
		paths, _ := doc["paths"].(map[string]any)
		item, ok := paths["/api/v0/supply-chain/impact/suppressions"].(map[string]any)
		if !ok {
			t.Errorf("origin %q: served spec lacks /api/v0/supply-chain/impact/suppressions", origin)
			continue
		}
		post, _ := item["post"].(map[string]any)
		if got := post["operationId"]; got != "createVulnerabilitySuppression" {
			t.Errorf("origin %q: suppressions post operationId = %v, want createVulnerabilitySuppression", origin, got)
		}
	}
}

// TestOpenAPINative_ServesAdminIdentityFamily pins the dest-served
// openapi.json for the admin-identity mutation family (satellite-design
// Decision 5(c)): the native route answers 200 with the fragment's three
// unique-path-key mutation paths and pinned operationIds, and it answers
// from compiled constants — no core fetch, so an unconfigured origin
// still serves the spec.
func TestOpenAPINative_ServesAdminIdentityFamily(t *testing.T) {
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
		for path, want := range map[string]map[string]string{
			"/api/v0/auth/local/invitations/{invite_id}/revoke":   {"post": "revokeAdminInvitation"},
			"/api/v0/auth/admin/role-assignments/revoke":          {"post": "revokeAdminRoleAssignment"},
			"/api/v0/auth/admin/idp-group-mappings/{mapping_ref}": {"delete": "deleteAdminIdPGroupMapping"},
		} {
			item, ok := paths[path].(map[string]any)
			if !ok {
				t.Errorf("origin %q: served spec lacks %s", origin, path)
				continue
			}
			for operation, wantID := range want {
				op, _ := item[operation].(map[string]any)
				if got := op["operationId"]; got != wantID {
					t.Errorf("origin %q: %s %s operationId = %v, want %s", origin, path, operation, got, wantID)
				}
			}
		}
	}
}

// TestOpenAPINative_ServesProviderConfigFamily pins the dest-served
// openapi.json for the admin provider-config mutation family
// (satellite-design Decision 5(c)): the native route answers 200 with
// the fragment's four unique-path-key mutation paths and pinned
// operationIds, and it answers from compiled constants — no core fetch,
// so an unconfigured origin still serves the spec. The create and
// update POSTs share path keys with the family GETs, so per the core
// OpenAPI convention they live in the reads fragment, which this lane
// does not port (reads stay relay).
func TestOpenAPINative_ServesProviderConfigFamily(t *testing.T) {
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
		for path, want := range map[string]map[string]string{
			"/api/v0/auth/admin/provider-configs/{provider_config_id}/revert":          {"post": "revertAdminProviderConfig"},
			"/api/v0/auth/admin/provider-configs/{provider_config_id}/enable":          {"post": "enableAdminProviderConfig"},
			"/api/v0/auth/admin/provider-configs/{provider_config_id}/disable":         {"post": "disableAdminProviderConfig"},
			"/api/v0/auth/admin/provider-configs/{provider_config_id}/test-connection": {"post": "testAdminProviderConfigConnection"},
		} {
			item, ok := paths[path].(map[string]any)
			if !ok {
				t.Errorf("origin %q: served spec lacks %s", origin, path)
				continue
			}
			for operation, wantID := range want {
				op, _ := item[operation].(map[string]any)
				if got := op["operationId"]; got != wantID {
					t.Errorf("origin %q: %s %s operationId = %v, want %s", origin, path, operation, got, wantID)
				}
			}
		}
	}
}

// TestOpenAPINative_ServesAdminRecoveryFamily pins the dest-served
// openapi.json for the admin recovery mutation family
// (satellite-design Decision 5(c)): the native route answers 200 with
// the fragment's four unique-path-key mutation paths and pinned
// summaries, and it answers from compiled constants — no core fetch,
// so an unconfigured origin still serves the spec. The relay-only
// four keep their own path keys in the pinned core file and stay out
// of this fragment (reads and relay-only mutations stay relay). The
// pinned fragment carries no operationId entries, so the paths are
// pinned by summary here.
func TestOpenAPINative_ServesAdminRecoveryFamily(t *testing.T) {
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
		for path, wantSummary := range map[string]string{
			"/api/v0/admin/replay":              "Replay failed work items",
			"/api/v0/admin/reopen":              "Reopen completed work items",
			"/api/v0/admin/refinalize":          "Refinalize scopes",
			"/api/v0/admin/recover-generations": "Recover wedged generations",
		} {
			item, ok := paths[path].(map[string]any)
			if !ok {
				t.Errorf("origin %q: served spec lacks %s", origin, path)
				continue
			}
			post, _ := item["post"].(map[string]any)
			if got := post["summary"]; got != wantSummary {
				t.Errorf("origin %q: %s post summary = %v, want %q", origin, path, got, wantSummary)
			}
		}
	}
}

// TestOpenAPINative_V1AliasServesSpec pins that the v1 alias rewrite lands
// the spec request on the same native handler: GET /api/v1/openapi.json
// answers the dest-assembled document with the v0 deprecation headers
// absent (the outer layer saw the v1 path).
func TestOpenAPINative_V1AliasServesSpec(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v1/openapi.json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(body), `"updateAdminSignInPolicy"`) {
		t.Errorf("v1 spec body lacks updateAdminSignInPolicy, want the native family section")
	}
	if !strings.Contains(string(body), `"/api/v0/admin/repository-retirements"`) {
		t.Errorf("v1 spec body lacks the retirement family path, want the native family section")
	}
	if !strings.Contains(string(body), `"Acknowledge a collector terminal failure"`) {
		t.Errorf("v1 spec body lacks the terminal-failure family section")
	}
	if !strings.Contains(string(body), `"revokeAdminInvitation"`) {
		t.Errorf("v1 spec body lacks the admin-identity family section")
	}
	if !strings.Contains(string(body), `"revertAdminProviderConfig"`) {
		t.Errorf("v1 spec body lacks the provider-config family section")
	}
}
