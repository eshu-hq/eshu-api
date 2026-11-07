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

// D9 SAML-login served-spec proof at the route level (Refs
// eshu-hq/eshu#9669, part of #6707). The openapi package tests pin the
// assembled document; these tests pin that the native openapi.json
// route serves it to keyed callers with the lane path keys present —
// for both a configured and an unconfigured core origin — while the
// key gate still fronts the route (the standing no-new-public-surface
// decision for the assembled spec).

// d9SAMLWaveSpecKeys maps every D9 SAML lane path key to the methods
// the served document must carry for it. Metadata and login carry
// exactly the GET their fragment entries document; ACS carries exactly
// the POST its fragment entry documents. The OIDC and GitHub keys stay
// in their own lane tables. The D5 wiring suite unions this table to
// prove every served GET is watched.
var d9SAMLWaveSpecKeys = map[string][]string{
	"/api/v0/auth/saml/providers/{provider_id}/metadata": {"get"},
	"/api/v0/auth/saml/providers/{provider_id}/login":    {"get"},
	"/api/v0/auth/saml/providers/{provider_id}/acs":      {"post"},
}

func TestD9SAMLWaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(d9SAMLWaveSpecKeys) != 3 {
		t.Fatalf("d9SAMLWaveSpecKeys holds %d keys, want the 3 served SAML path keys", len(d9SAMLWaveSpecKeys))
	}
}

// TestD9SAMLNative_ServesWaveSpecKeys pins that the native spec route
// serves the lane keys: authed callers get the dest-assembled document
// with every lane path and its pinned methods, with or without a core
// origin configured.
func TestD9SAMLNative_ServesWaveSpecKeys(t *testing.T) {
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
			t.Fatalf("origin %q: served spec is not valid JSON: %v", origin, body)
		}
		paths, _ := doc["paths"].(map[string]any)
		for path, methods := range d9SAMLWaveSpecKeys {
			item, ok := paths[path].(map[string]any)
			if !ok {
				t.Errorf("origin %q: served spec lacks lane key %s", origin, path)
				continue
			}
			if len(item) != len(methods) {
				t.Errorf("origin %q: %s serves %d methods, want %d (%v)", origin, path, len(item), len(methods), methods)
				continue
			}
			for _, method := range methods {
				if _, ok := item[method]; !ok {
					t.Errorf("origin %q: %s serves no %s operation", origin, path, method)
				}
			}
		}
	}
}
