// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Wave-1b ahead-of-relay RED (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#101). The 2 component-extension routes must
// terminate on the native lane for a shared-key caller. Until the
// lane lands, the calls cross the generic relay and this test is RED.
var compextRedLegs = []struct {
	method string
	path   string
	body   string
	family string
}{
	{http.MethodGet, "/api/v0/component-extensions", "", "other"},
	{http.MethodGet, "/api/v0/component-extensions/comp-1/diagnostics", "", "other"},
}

func compextAuthedReq(method, path, body string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	req.Header.Set("Accept", "application/json")
	return req
}

// TestCompextWaveAheadOfRelay pins native termination for both
// Wave-1b legs: a shared-key call reaches the stub core on the
// native lane with the wave path intact.
func TestCompextWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, leg := range compextRedLegs {
		t.Run(leg.method+" "+leg.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, compextAuthedReq(leg.method, leg.path, leg.body))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			if core.lastPath != leg.path {
				t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, leg.path)
			}
			snap := tel.snapshot()
			lanes, ok := snap.dispatches[leg.family]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", leg.family)
			}
			if lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", leg.family, lanes)
			}
			if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1", leg.family, got)
			}
		})
	}
}
