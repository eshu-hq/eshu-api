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

// G2 served-spec posture at the route level (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#101 RE:G2V2). The 7
// G2 routes mount natively in the G2 lane, but their Decision 5(c)
// fragments stay core-side for this lane: each entry lives in its
// owning core paths file (see api/g2_wave.go Fragment column), and
// porting the entries without their referenced blocks would break
// the verbatim-port rule scripts/check-parity.sh gates — so, like
// the Wave-1b lane, the routes are disclosed spec absences (see
// contracts/g2-wave.md), not a port. A follow-up spec-port lane
// claims the keys when it ports the entries entry-shaped with their
// referenced schemas; until then no served key duplicates a relayed
// route, and no mounted route promises what the document does not
// carry. The openapi package tests pin the assembled document;
// these tests pin that the native openapi.json route serves it with
// the wave keys disclosed-absent — for both a configured and an
// unconfigured core origin — while the key gate still fronts the
// route (the standing no-new-public-surface decision for the
// assembled spec).

// g2WaveSpecKeys are the path keys the G2 lane discloses absent
// from the served document: one entry per mounted wave route. No G2
// route is a served key in this lane.
var g2WaveSpecKeys = []string{
	"/api/v0/collector-extraction-readiness",
	"/api/v0/collector-extraction-readiness/{family}",
	"/api/v0/fact-schema-versions",
	"/api/v0/fact-schema-versions/{fact_kind}",
	"/api/v0/surface-inventory",
	"/api/v0/visualizations/derive",
	"/api/v0/repository/manifest-dependencies",
}

func TestG2WaveSpecKeyCount(t *testing.T) {
	t.Parallel()

	if len(g2WaveSpecKeys) != 7 {
		t.Fatalf("g2WaveSpecKeys holds %d keys, want the 7 disclosed-absent G2 keys", len(g2WaveSpecKeys))
	}
	if len(g2WaveSpecKeys) != len(g2Wave) {
		t.Fatalf("g2WaveSpecKeys holds %d keys, want the %d wave routes", len(g2WaveSpecKeys), len(g2Wave))
	}
}

// TestG2Native_SpecKeysDisclosedAbsent pins the G2 spec posture:
// the served document carries none of the 7 wave keys while the
// fragments stay core-side. Lifting an absence (porting the entry)
// belongs to the follow-up spec-port lane, not to the termination
// lane.
func TestG2Native_SpecKeysDisclosedAbsent(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"http://core.invalid", ""} {
		srv := NewServer(origin, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := g2AuthedReq(http.MethodGet, "/api/v0/openapi.json", "")
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
		for _, path := range g2WaveSpecKeys {
			if _, ok := paths[path]; ok {
				t.Errorf("origin %q: served spec carries %s, want it disclosed-absent until the follow-up spec-port lane", origin, path)
			}
		}
	}
}

// TestG2OpenAPIStillKeyGated pins that the assembled spec route
// stays behind the key gate: a credential-less caller gets the
// standing 401 even though the document is fully dest-served.
func TestG2OpenAPIStillKeyGated(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v0/openapi.json", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want the standing 401", rec.Code)
	}
}
