// SPDX-License-Identifier: MIT
// Copyright (c) 2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// relayLegCase is one re-pointed benchmark leg exercised through the
// shared constructor the benchmark itself uses, so deleting a Bearer
// header breaks this guard instead of silently restoring
// terminated-vs-terminated.
type relayLegCase struct {
	name  string
	build func(bearer string) *http.Request
}

// TestExplicitRelayLegsStayRelayed pins the relay lane for every
// re-pointed benchmark leg: the foreign-bearer request built by the
// benchmark's own constructor dispatches on viaRelay only with exactly
// one upstream fetch, while the shared-key request dispatches on
// viaNative only (also one fetch — the native lane still reads the
// core directly). All 18 legs verified green below.
func TestExplicitRelayLegsStayRelayed(t *testing.T) {
	legs := []relayLegCase{
		{name: "code-divergence/post", build: codeDivergenceRelayPost},
		{name: "code-graph/post", build: codeGraphRelayPost},
		{name: "code-search/post", build: codeSearchRelayPost},
		{name: "compare/post", build: compareRelayPost},
		{name: "d3/get", build: d3RelayGet},
		{name: "d3/post", build: d3RelayPost},
		{name: "d4/get", build: d4RelayGet},
		{name: "d4/post", build: d4RelayPost},
		{name: "d7/post", build: d7RelayPost},
		{name: "d8/post", build: d8RelayPost},
		{name: "d8/get", build: d8RelayGet},
		{name: "d9saml/post", build: d9SAMLRelayPost},
		{name: "dead-code/post", build: deadCodeRelayPost},
		{name: "iac/post", build: iacRelayPost},
		{name: "leafb/get", build: leafbRelayGet},
		{name: "leafb/post", build: leafbRelayPost},
		{name: "setup/post", build: setupRelayPost},
		{name: "setup/get", build: setupRelayGet},
	}

	for _, leg := range legs {
		t.Run(leg.name, func(t *testing.T) {
			serve := func(bearer string) (*stubCore, map[string]map[string]map[string]uint64) {
				core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, leg.build(bearer))
				return core, tel.snapshot().dispatches
			}

			core, dispatches := serve("foreign")
			if core.hits != 1 {
				t.Errorf("foreign-bearer leg %q reached the core %d times, want exactly 1 (explicit relay)", leg.name, core.hits)
			}
			foundRelay := false
			for family, lanes := range dispatches {
				if lanes[viaNative] != nil {
					t.Errorf("family %q files a native lane row for the foreign-bearer leg, want relay only", family)
				}
				if lanes[viaRelay] != nil {
					foundRelay = true
				}
			}
			if !foundRelay {
				t.Errorf("dispatches = %v, want a relay lane row for the foreign-bearer leg", dispatches)
			}

			sharedCore, sharedDispatches := serve(skeletonTestKey)
			if sharedCore.hits != 1 {
				t.Errorf("shared-key leg %q reached the core %d times, want exactly 1 (native lane reads core directly)", leg.name, sharedCore.hits)
			}
			foundNative := false
			for family, lanes := range sharedDispatches {
				if lanes[viaRelay] != nil {
					t.Errorf("family %q files a relay lane row for the shared-key leg, want native only", family)
				}
				if lanes[viaNative] != nil {
					foundNative = true
				}
			}
			if !foundNative {
				t.Errorf("dispatches = %v, want a native lane row for the shared-key leg", sharedDispatches)
			}
		})
	}
}
