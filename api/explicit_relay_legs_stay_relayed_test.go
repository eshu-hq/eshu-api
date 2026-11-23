// SPDX-License-Identifier: MIT
// Copyright (c) 2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// relayLeg describes one re-pointed benchmark leg: a request that must ride
// foreign-bearer explicit relay on its wave-mounted route now that the shared
// key terminates natively. Deleting the foreign bearer must flip the leg to
// native (zero upstream fetches), never silently pass.
type relayLeg struct {
	name        string
	method      string
	route       string
	body        string
	contentType string
}

// TestExplicitRelayLegsStayRelayed pins the relay-lane premise every
// re-pointed benchmark leg rests on: a foreign bearer on a wave-mounted
// route relays explicitly (exactly one upstream fetch, dispatch recorded
// on the relay lane only, never native), while the shared key terminates
// natively (zero upstream fetches). This is the same guard shape as
// TestContentRelayLegStaysRelayed, extended to all re-pointed legs so no
// single header edit can restore terminated-vs-terminated silently.
func TestExplicitRelayLegsStayRelayed(t *testing.T) {
	legs := []relayLeg{
		{name: "code-divergence/post", method: http.MethodPost, route: "/api/v0/impact/change-surface", body: `{"query":"q"}`},
		{name: "code-graph/post", method: http.MethodPost, route: "/api/v0/impact/change-surface", body: `{"query":"q"}`},
		{name: "code-search/post", method: http.MethodPost, route: "/api/v0/impact/change-surface", body: `{"query":"render"}`},
		{name: "compare/post", method: http.MethodPost, route: "/api/v0/impact/change-surface", body: compareParityBody},
		{name: "d3/get", method: http.MethodGet, route: "/api/v0/dependencies"},
		{name: "d3/post", method: http.MethodPost, route: "/api/v0/impact/change-surface", body: `{"question":"q"}`},
		{name: "d4/get", method: http.MethodGet, route: "/api/v0/dependencies"},
		{name: "d4/post", method: http.MethodPost, route: "/api/v0/impact/change-surface", body: `{"surface":"x"}`},
		{name: "d7/post", method: http.MethodPost, route: "/api/v0/impact/change-surface"},
		{name: "d8/post", method: http.MethodPost, route: "/api/v0/impact/change-surface"},
		{name: "d8/get", method: http.MethodGet, route: "/api/v0/dependencies"},
		{name: "d9saml/post", method: http.MethodPost, route: "/api/v0/impact/change-surface", body: d9SAMLACSLargeForm, contentType: "application/x-www-form-urlencoded"},
		{name: "dead-code/post", method: http.MethodPost, route: "/api/v0/impact/change-surface", body: `{"query":"q"}`},
		{name: "iac/post", method: http.MethodPost, route: "/api/v0/impact/change-surface"},
		{name: "leafb/get", method: http.MethodGet, route: "/api/v0/dependencies"},
		{name: "leafb/post", method: http.MethodPost, route: "/api/v0/impact/change-surface", body: `{"question":"q"}`},
		{name: "setup/post", method: http.MethodPost, route: "/api/v0/impact/change-surface"},
		{name: "setup/get", method: http.MethodGet, route: "/api/v0/dependencies"},
	}

	for _, leg := range legs {
		t.Run(leg.name, func(t *testing.T) {
			body := leg.body
			contentType := leg.contentType

			build := func(bearer string) *http.Request {
				var reader io.Reader
				if body != "" {
					reader = strings.NewReader(body)
				}
				req := httptest.NewRequest(leg.method, leg.route, reader)
				if contentType != "" {
					req.Header.Set("Content-Type", contentType)
				}
				req.Header.Set("Authorization", "Bearer "+bearer)
				return req
			}

			serve := func(bearer string) (*stubCore, telemetrySnapshot) {
				core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, build(bearer))
				return core, tel.snapshot()
			}

			core, snap := serve("foreign")
			if core.hits != 1 {
				t.Errorf("foreign-bearer leg %q reached the core %d times, want exactly 1 (explicit relay)", leg.name, core.hits)
			}
			foundRelay := false
			for family, lanes := range snap.dispatches {
				if lanes[viaNative] != nil {
					t.Errorf("family %q files a native lane row for the foreign-bearer leg, want relay only", family)
				}
				if lanes[viaRelay] != nil {
					foundRelay = true
				}
			}
			if !foundRelay {
				t.Errorf("dispatches = %v, want a relay lane row for the foreign-bearer leg", snap.dispatches)
			}

			sharedCore, sharedSnap := serve(skeletonTestKey)
			if sharedCore.hits != 1 {
				t.Errorf("shared-key leg %q reached the core %d times, want exactly 1 (native lane still reads core directly)", leg.name, sharedCore.hits)
			}
			foundNative := false
			for family, lanes := range sharedSnap.dispatches {
				if lanes[viaRelay] != nil {
					t.Errorf("family %q files a relay lane row for the shared-key leg, want native only", family)
				}
				if lanes[viaNative] != nil {
					foundNative = true
				}
			}
			if !foundNative {
				t.Errorf("dispatches = %v, want a native lane row for the shared-key leg", sharedSnap.dispatches)
			}
		})
	}
}
