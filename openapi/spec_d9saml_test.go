// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// D9 SAML-login spec proof (Refs eshu-hq/eshu#9669, part of #6707).
// The D9 SAML lane ports one line-verbatim subset (the metadata and
// login GETs plus the ACS POST) into the served document, referencing
// only the BrowserSessionResponse schema the D8 lane already ported —
// no new schemas. Byte identity of every ported line rides
// scripts/check-parity.sh; these tests pin the served assembly around
// those bytes. The OIDC and GitHub keys stay in their own lane tests.

// TestSpec_ServesD9SAMLFamilies pins the native-spec contract for the
// D9 SAML-login family: the assembled document is valid JSON and
// carries every lane path key with its pinned summary, and exactly the
// method each fragment entry documents.
func TestSpec_ServesD9SAMLFamilies(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantOps := range map[string]map[string]string{
		"/api/v0/auth/saml/providers/{provider_id}/metadata": {
			"get": "Read SAML service-provider metadata",
		},
		"/api/v0/auth/saml/providers/{provider_id}/login": {
			"get": "Start SAML login",
		},
		"/api/v0/auth/saml/providers/{provider_id}/acs": {
			"post": "Complete SAML login at the assertion consumer service",
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

// TestSpec_D9SAMLFamilySectionMatchesFragment pins lossless assembly
// for the D9 SAML keys: the served path objects equal the ported
// fragment's parsed path objects, so the only reviewed delta to the
// pinned core fragment is the fragment file itself (gated line-verbatim
// by scripts/check-parity.sh).
func TestSpec_D9SAMLFamilySectionMatchesFragment(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)

	for _, path := range []string{
		"/api/v0/auth/saml/providers/{provider_id}/metadata",
		"/api/v0/auth/saml/providers/{provider_id}/login",
		"/api/v0/auth/saml/providers/{provider_id}/acs",
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
