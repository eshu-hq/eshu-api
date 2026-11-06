// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSpec_ServesSignInPolicyFamily pins the native-spec contract for the
// sign-in-policy family (satellite-design Decision 5(c)): the assembled
// document is valid JSON, carries both family paths with the pinned
// operationIds, and embeds the ported fragment losslessly.
func TestSpec_ServesSignInPolicyFamily(t *testing.T) {
	t.Parallel()

	var doc map[string]any
	if err := json.Unmarshal([]byte(Spec()), &doc); err != nil {
		t.Fatalf("Spec() is not valid JSON: %v", err)
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("Spec() paths = %T, want object", doc["paths"])
	}

	public, ok := paths["/api/v0/auth/sign-in-policy"].(map[string]any)
	if !ok {
		t.Fatalf("paths lacks /api/v0/auth/sign-in-policy: %v", keys(paths))
	}
	if got := operationID(public, "get"); got != "getPublicSignInPolicy" {
		t.Errorf("public sign-in-policy get operationId = %q, want getPublicSignInPolicy", got)
	}

	admin, ok := paths["/api/v0/auth/admin/sign-in-policy"].(map[string]any)
	if !ok {
		t.Fatalf("paths lacks /api/v0/auth/admin/sign-in-policy: %v", keys(paths))
	}
	if got := operationID(admin, "get"); got != "getAdminSignInPolicy" {
		t.Errorf("admin sign-in-policy get operationId = %q, want getAdminSignInPolicy", got)
	}
	if got := operationID(admin, "patch"); got != "updateAdminSignInPolicy" {
		t.Errorf("admin sign-in-policy patch operationId = %q, want updateAdminSignInPolicy", got)
	}
}

// TestSpec_ServesRetirementFamily pins the native-spec contract for the
// retirement family (satellite-design Decision 5(c)): the assembled
// document is valid JSON, carries the family path, and embeds the ported
// fragment losslessly.
func TestSpec_ServesRetirementFamily(t *testing.T) {
	t.Parallel()

	var doc map[string]any
	if err := json.Unmarshal([]byte(Spec()), &doc); err != nil {
		t.Fatalf("Spec() is not valid JSON: %v", err)
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("Spec() paths = %T, want object", doc["paths"])
	}

	retire, ok := paths["/api/v0/admin/repository-retirements"].(map[string]any)
	if !ok {
		t.Fatalf("paths lacks /api/v0/admin/repository-retirements: %v", keys(paths))
	}
	post, ok := retire["post"].(map[string]any)
	if !ok {
		t.Fatalf("retirement path lacks post: %v", retire)
	}
	if got := post["summary"]; got != "Retire repositories: mark, fence, then purge their indexed state" {
		t.Errorf("retirement post summary = %q, want the pinned summary", got)
	}
}

// TestSpec_ServesTerminalFailureFamily pins the native-spec contract for
// the terminal-failure family (satellite-design Decision 5(c)): the
// assembled document is valid JSON and carries all three family paths —
// the acknowledge POST and its two query POSTs. The pinned core fragment
// carries no operationId entries (unlike the sign-in-policy fragment),
// so this test pins the family paths by their summaries; the byte-identity
// gate in scripts/check-parity.sh is what keeps the fragment lossless,
// and TestSpec_FamilySectionMatchesFragment pins that the served paths
// equal the ported fragment.
func TestSpec_ServesTerminalFailureFamily(t *testing.T) {
	t.Parallel()

	var doc map[string]any
	if err := json.Unmarshal([]byte(Spec()), &doc); err != nil {
		t.Fatalf("Spec() is not valid JSON: %v", err)
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("Spec() paths = %T, want object", doc["paths"])
	}

	for path, wantSummary := range map[string]string{
		"/api/v0/admin/collector-failures/acknowledge":            "Acknowledge a collector terminal failure",
		"/api/v0/admin/collector-failures/acknowledgements/query": "List collector terminal failure acknowledgements",
		"/api/v0/admin/collector-failures/query":                  "List the failed scopes of one collector instance",
	} {
		item, ok := paths[path].(map[string]any)
		if !ok {
			t.Errorf("paths lacks %s: %v", path, keys(paths))
			continue
		}
		op, _ := item["post"].(map[string]any)
		if got := op["summary"]; got != wantSummary {
			t.Errorf("%s post summary = %v, want %q", path, got, wantSummary)
		}
		if _, hasOpID := op["operationId"]; hasOpID {
			t.Errorf("%s post carries operationId %v, want none (the pinned fragment has none)", path, op["operationId"])
		}
	}
}

// TestSpec_ServesAdminIdentityFamily pins the native-spec contract for
// the admin-identity mutation family (satellite-design Decision 5(c)):
// the assembled document is valid JSON and carries the three fragment
// paths whose keys are unique to the mutation surface — the invitation
// revoke, the role-assignment revoke, and the mapping delete — with
// their pinned operationIds. The grant and create POSTs share path keys
// with the family GETs, so per the core OpenAPI convention they live in
// the reads fragment (ported alongside in this union; reads stay relay);
// the lossless-assembly test below pins that the served paths equal the
// ported fragment, and scripts/check-parity.sh gates its byte-identity.
func TestSpec_ServesAdminIdentityFamily(t *testing.T) {
	servesAdminIdentityFamily(t)
}

// TestSpec_ServesAdminIdentityReadsFamily pins the native-spec contract
// for the admin-identity reads family (satellite-design Decision 5(c)):
// the assembled document is valid JSON and carries all seven fragment
// path keys with their pinned operationIds — the seven read-list GETs
// plus the grant and create POSTs that share path keys with the family
// GETs (per the core OpenAPI convention they live in this reads
// fragment, not the mutations fragment; reads stay relay). The
// lossless-assembly test below pins that the served paths equal the
// ported fragment, and scripts/check-parity.sh gates its byte-identity.
func TestSpec_ServesAdminIdentityReadsFamily(t *testing.T) {
	servesAdminIdentityReadsFamily(t)
}

// servesAdminIdentityFamily holds the mutation-family assertions for the
// union rebase: the assembled document is valid JSON and carries the three
// mutation-surface paths with their pinned operationIds.
func servesAdminIdentityFamily(t *testing.T) {
	t.Helper()
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/auth/local/invitations/{invite_id}/revoke":   {"post": "revokeAdminInvitation"},
		"/api/v0/auth/admin/role-assignments/revoke":          {"post": "revokeAdminRoleAssignment"},
		"/api/v0/auth/admin/idp-group-mappings/{mapping_ref}": {"delete": "deleteAdminIdPGroupMapping"},
	})
}

// servesAdminIdentityReadsFamily holds the reads-family assertions for the
// union rebase: the assembled document is valid JSON and carries all seven
// reads-fragment path keys with their pinned operationIds.
func servesAdminIdentityReadsFamily(t *testing.T) {
	t.Helper()
	assertServedPaths(t, map[string]map[string]string{
		"/api/v0/auth/admin/role-assignments":   {"get": "listAdminRoleAssignments", "post": "grantAdminRoleAssignment"},
		"/api/v0/auth/admin/roles":              {"get": "listAdminRoles"},
		"/api/v0/auth/admin/idp-providers":      {"get": "listAdminIdPProviders"},
		"/api/v0/auth/admin/idp-group-mappings": {"get": "listAdminIdPGroupMappings", "post": "createAdminIdPGroupMapping"},
		"/api/v0/auth/admin/api-tokens":         {"get": "listAdminAPITokens"},
		"/api/v0/auth/admin/audit/events":       {"get": "listAdminAuditEvents"},
		"/api/v0/auth/admin/audit/summary":      {"get": "summarizeAdminAuditEvents"},
	})
}

// assertServedPaths checks that Spec() is valid JSON and carries every
// path in want with its pinned operationIds.
func assertServedPaths(t *testing.T, want map[string]map[string]string) {
	t.Helper()
	t.Parallel()

	var doc map[string]any
	if err := json.Unmarshal([]byte(Spec()), &doc); err != nil {
		t.Fatalf("Spec() is not valid JSON: %v", err)
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("Spec() paths = %T, want object", doc["paths"])
	}

	for path, wantOps := range want {
		item, ok := paths[path].(map[string]any)
		if !ok {
			t.Errorf("paths lacks %s: %v", path, keys(paths))
			continue
		}
		for operation, wantID := range wantOps {
			op, _ := item[operation].(map[string]any)
			if got := op["operationId"]; got != wantID {
				t.Errorf("%s %s operationId = %v, want %s", path, operation, got, wantID)
			}
		}
	}
}

// TestSpec_ServesProviderConfigFamily pins the native-spec contract for
// the admin provider-config mutation family (satellite-design Decision
// 5(c)): the assembled document is valid JSON and carries the four
// fragment paths whose keys are unique to the mutation surface — the
// revert, enable, disable, and test-connection POSTs — with their pinned
// operationIds. The create and update POSTs share path keys with the
// family list/detail GETs, so per the core OpenAPI convention they live
// in the reads fragment, which this lane does not port (reads stay
// relay); the lossless-assembly test below pins that the served paths
// equal the ported fragment, and scripts/check-parity.sh gates its
// line-verbatim provenance (a whole-file identity port is impossible:
// core ships reads and mutations in one file, so this lane ports the
// unique-key mutation path objects as a line-verbatim subset).
func TestSpec_ServesProviderConfigFamily(t *testing.T) {
	t.Parallel()

	var doc map[string]any
	if err := json.Unmarshal([]byte(Spec()), &doc); err != nil {
		t.Fatalf("Spec() is not valid JSON: %v", err)
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("Spec() paths = %T, want object", doc["paths"])
	}

	for path, want := range map[string]map[string]string{
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/revert":          {"post": "revertAdminProviderConfig"},
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/enable":          {"post": "enableAdminProviderConfig"},
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/disable":         {"post": "disableAdminProviderConfig"},
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/test-connection": {"post": "testAdminProviderConfigConnection"},
	} {
		item, ok := paths[path].(map[string]any)
		if !ok {
			t.Errorf("paths lacks %s: %v", path, keys(paths))
			continue
		}
		for operation, wantID := range want {
			op, _ := item[operation].(map[string]any)
			if got := op["operationId"]; got != wantID {
				t.Errorf("%s %s operationId = %v, want %s", path, operation, got, wantID)
			}
		}
	}
}

// TestSpec_ServesAdminRecoveryFamily pins the native-spec contract for
// the admin recovery mutation family (satellite-design Decision 5(c)):
// the assembled document is valid JSON and carries the four fragment
// paths unique to the tier C-c2 lane — the replay, reopen, refinalize,
// and recover-generations POSTs, pinned by summary (the fragment has
// no operationIds). The relay-only four keep their own path keys and
// stay out, so no served key duplicates a relayed route; the
// lossless-assembly test and check-parity.sh gate the rest.
func TestSpec_ServesAdminRecoveryFamily(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantSummary := range map[string]string{
		"/api/v0/admin/replay":              "Replay failed work items",
		"/api/v0/admin/reopen":              "Reopen completed work items",
		"/api/v0/admin/refinalize":          "Refinalize scopes",
		"/api/v0/admin/recover-generations": "Recover wedged generations",
	} {
		item, ok := served[path].(map[string]any)
		if !ok {
			t.Errorf("paths lacks %s: %v", path, keys(served))
			continue
		}
		op, _ := item["post"].(map[string]any)
		if got := op["summary"]; got != wantSummary {
			t.Errorf("%s post summary = %v, want %q", path, got, wantSummary)
		}
		if _, hasOpID := op["operationId"]; hasOpID {
			t.Errorf("%s post carries operationId %v, want none (the pinned fragment has none)", path, op["operationId"])
		}
	}
}

// TestSpec_FamilySectionMatchesFragment pins lossless assembly: the served
// family path objects equal the ported fragments' parsed path objects, so
// the only reviewed delta to the pinned core fragments is the fragment files
// themselves (gated byte-identical by scripts/check-parity.sh).
func TestSpec_FamilySectionMatchesFragment(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)

	for _, path := range []string{
		"/api/v0/auth/sign-in-policy",
		"/api/v0/auth/admin/sign-in-policy",
		"/api/v0/admin/repository-retirements",
		"/api/v0/admin/collector-failures/acknowledge",
		"/api/v0/admin/collector-failures/acknowledgements/query",
		"/api/v0/admin/collector-failures/query",
		"/api/v0/auth/local/invitations/{invite_id}/revoke",
		"/api/v0/auth/admin/role-assignments/revoke",
		"/api/v0/auth/admin/idp-group-mappings/{mapping_ref}",
		"/api/v0/auth/admin/role-assignments",
		"/api/v0/auth/admin/roles",
		"/api/v0/auth/admin/idp-providers",
		"/api/v0/auth/admin/idp-group-mappings",
		"/api/v0/auth/admin/api-tokens",
		"/api/v0/auth/admin/audit/events",
		"/api/v0/auth/admin/audit/summary",
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/revert",
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/enable",
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/disable",
		"/api/v0/auth/admin/provider-configs/{provider_config_id}/test-connection",
		"/api/v0/admin/replay",
		"/api/v0/admin/reopen",
		"/api/v0/admin/refinalize",
		"/api/v0/admin/recover-generations",
		"/api/v0/supply-chain/impact/suppressions",
	} {
		want, ok := fragment[path]
		if !ok {
			t.Fatalf("fragment lacks %s", path)
		}
		got, ok := served[path]
		if !ok {
			t.Fatalf("served spec lacks %s", path)
		}
		if !jsonEqual(got, want) {
			t.Errorf("served %s differs from the ported fragment", path)
		}
	}
}

// TestSpec_LocalRefsResolve pins that every local $ref the family
// fragments emit resolves inside the served document: the minimal
// components block must carry every schema and response the fragments
// name (ErrorResponse plus BadRequest, Forbidden, Conflict,
// InternalError, and ServiceUnavailable for the terminal-failure
// fragment; NotFound for the admin-identity mutations fragment;
// AdminAuditCountList for the admin-identity reads fragment; NotFound
// and AdminProviderConfigWriteResult for the provider-config fragment;
// SkippedScopesReport, DeltaActiveScopesReport, and
// ReindexRequestsWrittenReport for the admin-recovery fragment;
// AdminSignInPolicy schemas for the sign-in-policy family; the shared
// ServiceUnavailable response for the suppression fragment).
func TestSpec_LocalRefsResolve(t *testing.T) {
	t.Parallel()

	raw := Spec()
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("Spec() is not valid JSON: %v", err)
	}

	components, ok := doc["components"].(map[string]any)
	if !ok {
		t.Fatalf("Spec() components = %T, want object", doc["components"])
	}

	var missing []string
	var walk func(value any)
	walk = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			for key, item := range node {
				if key == "$ref" {
					target, _ := item.(string)
					if !strings.HasPrefix(target, "#/components/") {
						missing = append(missing, target)
						continue
					}
					rest := strings.TrimPrefix(target, "#/components/")
					parts := strings.SplitN(rest, "/", 2)
					if len(parts) != 2 {
						missing = append(missing, target)
						continue
					}
					section, _ := components[parts[0]].(map[string]any)
					if _, ok := section[parts[1]]; !ok {
						missing = append(missing, target)
					}
					continue
				}
				walk(item)
			}
		case []any:
			for _, item := range node {
				walk(item)
			}
		}
	}
	walk(doc["paths"])
	if len(missing) != 0 {
		t.Errorf("unresolved local $refs: %v", missing)
	}
}

// TestServeSpec_WritesDocument pins the HTTP shape: 200, the JSON
// document content type, open CORS (matching the core mount), and a body
// equal to Spec().
func TestServeSpec_WritesDocument(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v0/openapi.json", nil)
	rec := httptest.NewRecorder()
	ServeSpec(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want the JSON document shape", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want open CORS like the core mount", got)
	}
	body, _ := io.ReadAll(res.Body)
	if string(body) != Spec() {
		t.Errorf("served body differs from Spec()")
	}
}

// TestSpec_VersionPinned pins the native-spec contract version: v0 serves
// exactly the families whose fragments have landed (sign-in-policy first,
// retirement second, terminal-failure third, admin-identity mutations
// fourth, admin-identity reads fifth, provider-config sixth,
// admin-recovery seventh, suppression eighth). It advances as family
// lanes port their fragments.
func TestSpec_VersionPinned(t *testing.T) {
	t.Parallel()

	if SpecVersion != "v0" {
		t.Errorf("SpecVersion = %q, want v0 (bump with the served family set)", SpecVersion)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(Spec()), &doc); err != nil {
		t.Fatalf("Spec() is not valid JSON: %v", err)
	}
	info, _ := doc["info"].(map[string]any)
	if got := info["version"]; got != SpecVersion {
		t.Errorf("info.version = %v, want SpecVersion %q", got, SpecVersion)
	}
}

func operationID(pathItem map[string]any, method string) string {
	op, _ := pathItem[method].(map[string]any)
	id, _ := op["operationId"].(string)
	return id
}

func keys(pathItem map[string]any) []string {
	out := make([]string, 0, len(pathItem))
	for key := range pathItem {
		out = append(out, key)
	}
	return out
}

func decodePaths(t *testing.T, raw string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("decode paths: %v", err)
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("decoded paths = %T, want object", doc["paths"])
	}
	return paths
}

func jsonEqual(left, right any) bool {
	leftRaw, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightRaw, err := json.Marshal(right)
	if err != nil {
		return false
	}
	var leftNorm, rightNorm any
	if err := json.Unmarshal(leftRaw, &leftNorm); err != nil {
		return false
	}
	if err := json.Unmarshal(rightRaw, &rightNorm); err != nil {
		return false
	}
	leftCanon, _ := json.Marshal(leftNorm)
	rightCanon, _ := json.Marshal(rightNorm)
	return string(leftCanon) == string(rightCanon)
}
