// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// D9 GitHub-login spec proof (Refs eshu-hq/eshu#9669, part of #6707).
// The D9 GitHub lane ports one line-verbatim subset (the GitHub login
// + callback entries) into the served document, referencing only the
// BrowserSessionResponse schema the D8 lane already ported — no new
// schemas. Byte identity of every ported line rides
// scripts/check-parity.sh; these tests pin the served assembly around
// those bytes. The OIDC keys stay in their own lane tests and the SAML
// keys ride their own landed lane tests (spec_d9saml_test.go).

// TestSpec_ServesD9GitHubFamilies pins the native-spec contract for the
// D9 GitHub-login family: the assembled document is valid JSON and
// carries every lane path key with its pinned summary, and exactly the
// GET method each fragment entry documents.
func TestSpec_ServesD9GitHubFamilies(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantOps := range map[string]map[string]string{
		"/api/v0/auth/github/login": {
			"get": "Start a dashboard GitHub login (issue #5166, F-5)",
		},
		"/api/v0/auth/github/callback": {
			"get": "Complete a dashboard GitHub login (issue #5166, F-5)",
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

// TestSpec_D9GitHubFamilySectionMatchesFragment pins lossless assembly
// for the D9 GitHub keys: the served path objects equal the ported
// fragment's parsed path objects, so the only reviewed delta to the
// pinned core fragment is the fragment file itself (gated line-verbatim
// by scripts/check-parity.sh).
func TestSpec_D9GitHubFamilySectionMatchesFragment(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)

	for _, path := range []string{
		"/api/v0/auth/github/login",
		"/api/v0/auth/github/callback",
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
