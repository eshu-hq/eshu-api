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

// TestSpec_LocalRefsResolve pins that every local $ref the family fragment
// emits resolves inside the served document: the minimal components block
// must carry every schema and response the fragment names.
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
// retirement second). It advances as family lanes port their fragments.
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
