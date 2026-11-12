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

// W8a satellite termination served-spec posture (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#100). The 16 W8a routes mount natively,
// but their Decision 5(c) fragments stay core-side for this lane (the
// same disclosed-absence posture the Compare gap-04 lane records in
// contracts/compare-termination.md): a byte-identical satellite port
// cannot concatenate mid-list, and a re-shaped satellite subset
// would break the verbatim-port rule scripts/check-parity.sh gates.
// The follow-up spec-port lane claims the keys when core re-shapes
// the fragments or the lane ports them entry-shaped; until then no
// served key duplicates a relayed route, and no mounted route
// promises what the document does not carry.

// w8aDisclosedSpecKeys are the path keys the W8a lane mounts
// natively while their fragments stay core-side: the served
// document omits them by disclosure, pinned by
// openapi/spec_w8a_test.go. The three Status keys (governance,
// answer-narration, hosted-readiness) moved here from d6RelayedSpecKeys
// when the W8a lane terminated them natively.
var w8aDisclosedSpecKeys = []string{
	"/api/v0/secrets-iam/identity-trust-chains",
	"/api/v0/secrets-iam/privilege-posture-observations",
	"/api/v0/secrets-iam/secret-access-paths",
	"/api/v0/secrets-iam/posture-gaps",
	"/api/v0/secrets-iam/posture-summary",
	"/api/v0/status/governance",
	"/api/v0/status/answer-narration",
	"/api/v0/status/hosted-readiness",
	"/api/v0/aws/runtime-drift/findings",
	"/api/v0/replatforming/selectors",
	"/api/v0/replatforming/rollups",
	"/api/v0/replatforming/plans",
	"/api/v0/replatforming/ownership-packets",
	"/api/v0/metrics/timeseries",
	"/api/v0/search/semantic",
	"/api/v0/terraform/config-state-drift/findings",
}

// TestW8aDisclosedSpecKeyCount pins the W8a disclosed set: one entry
// per natively mounted wave route. A wave change moves this list
// with the ownership note, the fragments, and the wave mirror
// together.
func TestW8aDisclosedSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(w8aDisclosedSpecKeys) != 16 {
		t.Fatalf("w8aDisclosedSpecKeys holds %d keys, want the 16 disclosed W8a keys (one per wave route): move the key with the owning lane", len(w8aDisclosedSpecKeys))
	}
	if len(w8aDisclosedSpecKeys) != len(w8aWave) {
		t.Fatalf("w8aDisclosedSpecKeys holds %d keys for %d wave routes, want one disclosed key per wave route", len(w8aDisclosedSpecKeys), len(w8aWave))
	}
}

// TestW8aNative_OmitsDisclosedSpecKeys pins that the native spec
// route omits the disclosed keys: authed callers get the
// dest-assembled document without the W8a path keys, with or without
// a core origin configured.
func TestW8aNative_OmitsDisclosedSpecKeys(t *testing.T) {
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
		for _, path := range w8aDisclosedSpecKeys {
			if _, ok := paths[path]; ok {
				t.Errorf("origin %q: served spec carries disclosed key %s, want it out until the spec-port lane", origin, path)
			}
		}
	}
}
