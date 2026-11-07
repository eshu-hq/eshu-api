// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// D9 OIDC-login spec proof (Refs eshu-hq/eshu#9669, part of #6707).
// The D9 lane ports one line-verbatim subset (the OIDC login +
// callback entries) into the served document, referencing only the
// BrowserSessionResponse schema the D8 lane already ported — no new
// schemas. Byte identity of every ported line rides
// scripts/check-parity.sh; these tests pin the served assembly around
// those bytes. GitHub rides its own lane tests and the SAML keys ride
// their own landed lane tests (spec_d9saml_test.go).

// TestSpec_ServesD9Families pins the native-spec contract for the D9
// OIDC-login family: the assembled document is valid JSON and carries
// every lane path key with its pinned summary, and exactly the GET
// method each fragment entry documents.
func TestSpec_ServesD9Families(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantOps := range map[string]map[string]string{
		"/api/v0/auth/oidc/login": {
			"get": "Start a dashboard OIDC login",
		},
		"/api/v0/auth/oidc/callback": {
			"get": "Complete a dashboard OIDC login",
		},
	} {
		item, ok := served[path].(map[string]any)
		if !ok {
			t.Errorf("paths lacks %s: %v", path, keys(served))
			continue
		}
		if len(item) != len(wantOps) {
			t.Errorf("%s serves %d methods, want %d (%v)", path, len(item), len(wantOps), keys(item))
			continue
		}
		for method, wantSummary := range wantOps {
			op, _ := item[method].(map[string]any)
			if got := op["summary"]; got != wantSummary {
				t.Errorf("%s %s summary = %v, want %q", path, method, got, wantSummary)
			}
		}
	}
}

// TestSpec_D9FamilySectionMatchesFragment pins lossless assembly for
// the D9 keys: the served path objects equal the ported fragment's
// parsed path objects, so the only reviewed delta to the pinned core
// fragment is the fragment file itself (gated line-verbatim by
// scripts/check-parity.sh).
func TestSpec_D9FamilySectionMatchesFragment(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)

	for _, path := range []string{
		"/api/v0/auth/oidc/login",
		"/api/v0/auth/oidc/callback",
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
