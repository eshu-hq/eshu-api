// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

// D2 Leaf B Native-now conformance (Refs eshu-hq/eshu#9669, part of
// #6707). TDD record: this suite is RED as committed — the frozen wave
// below mirrors eshu's nativeNowWave
// (go/cmd/api/read_transport_conformance.go at the provenance pin) and
// the behavior tests require every wave route to terminate natively for
// shared-key callers while relaying unresolvable identity classes
// explicitly. Until the lane lands, the wave registers nothing and every
// wave call crosses the generic byte-relay (or the key gate), so the
// ahead-of-relay assertions fail first. That red is the recorded
// expectation, not a defect.

// TestLeafBWaveFrozen pins the exact Native-now first-wave surface the
// satellite must terminate. A wave change (route added, removed,
// renamed, or rehomed) fails here until the eshu spec, the eshu
// skeleton table, and this mirror move together — the same frozen-set
// pattern as TestCoreReadContractsVersioned. The want table is the
// verbatim eshu nativeNowWave (method, path, family) plus the
// satellite serve kind and the Decision 5(c) fragment that carries the
// route's spec entry.
func TestLeafBWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/openapi.json api_router landed paths/status/routes.go",
		"GET /health api_router landed <mux-health>",
		"GET /api/v0/docs api_router wave-public <doc-ui>",
		"GET /api/v0/redoc api_router wave-public <doc-ui>",
		"GET /api/v0/capabilities capabilities wave-gated paths/catalog/capabilities.go",
		"POST /api/v0/answer-quality/score cli_support wave-gated paths/cli/score.go",
		"POST /api/v0/investigation/packet/render cli_support wave-gated paths/cli/render.go",
		"POST /api/v0/report/bundle/capture cli_support wave-gated paths/cli/capture.go",
		"POST /api/v0/report/bundle/validate cli_support wave-gated paths/cli/validate.go",
		"POST /api/v0/service/report/compose cli_support wave-gated paths/cli/compose.go",
		"POST /api/v0/ask ask wave-gated paths/search/ask.go",
		"GET /api/v0/services/{service_name}/intelligence-report service_intel wave-gated paths/service/intelligence_report.go",
		"GET /api/v0/auth/providers auth_providers wave-public paths/auth/routes.go",
		"GET /api/v0/auth/sign-in-policy sign_in_policy wave-public paths/auth/sign_in_policy.go",
		"GET /api/v0/auth/admin/sign-in-policy sign_in_policy wave-gated paths/auth/sign_in_policy.go",
		"GET /api/v0/auth/admin/provider-configs provider_config wave-gated paths/auth/admin_provider_configs.go",
		"GET /api/v0/auth/admin/provider-configs/{provider_config_id} provider_config wave-gated paths/auth/admin_provider_configs.go",
		"GET /api/v0/auth/admin/provider-configs/{provider_config_id}/revisions provider_config wave-gated paths/auth/admin_provider_configs.go",
	}
	if len(leafbWave) != len(want) {
		t.Fatalf("leafbWave holds %d routes, want frozen %d: move the spec, the skeleton table, and this mirror together",
			len(leafbWave), len(want))
	}
	got := make([]string, 0, len(leafbWave))
	for _, route := range leafbWave {
		got = append(got, route.Method+" "+route.Path+" "+route.Family+" "+route.Serve+" "+route.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("leafbWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// leafbConcretePaths fills the wave's path wildcards so the behavior
// tests drive real mux matches. Trailing-slash spellings ride the same
// native registration (never-split), so each entry also names its slash
// variant where the mux distinguishes it.
var leafbConcretePaths = map[string]string{
	"GET /api/v0/openapi.json":                                               "/api/v0/openapi.json",
	"GET /health":                                                            "/health",
	"GET /api/v0/docs":                                                       "/api/v0/docs",
	"GET /api/v0/redoc":                                                      "/api/v0/redoc",
	"GET /api/v0/capabilities":                                               "/api/v0/capabilities",
	"POST /api/v0/answer-quality/score":                                      "/api/v0/answer-quality/score",
	"POST /api/v0/investigation/packet/render":                               "/api/v0/investigation/packet/render",
	"POST /api/v0/report/bundle/capture":                                     "/api/v0/report/bundle/capture",
	"POST /api/v0/report/bundle/validate":                                    "/api/v0/report/bundle/validate",
	"POST /api/v0/service/report/compose":                                    "/api/v0/service/report/compose",
	"POST /api/v0/ask":                                                       "/api/v0/ask",
	"GET /api/v0/services/{service_name}/intelligence-report":                "/api/v0/services/checkout/intelligence-report",
	"GET /api/v0/auth/providers":                                             "/api/v0/auth/providers?tenant_id=tenant_a",
	"GET /api/v0/auth/sign-in-policy":                                        "/api/v0/auth/sign-in-policy?tenant_id=tenant_a",
	"GET /api/v0/auth/admin/sign-in-policy":                                  "/api/v0/auth/admin/sign-in-policy",
	"GET /api/v0/auth/admin/provider-configs":                                "/api/v0/auth/admin/provider-configs",
	"GET /api/v0/auth/admin/provider-configs/{provider_config_id}":           "/api/v0/auth/admin/provider-configs/prov_1",
	"GET /api/v0/auth/admin/provider-configs/{provider_config_id}/revisions": "/api/v0/auth/admin/provider-configs/prov_1/revisions",
}

// TestLeafBWaveRoutesAheadOfRelay pins that every wave route is an
// explicit registration ahead of the generic proxy: a shared-key call
// must file its dispatch on the native lane (viaNative), not the relay
// lane. Today the wave registers nothing, so every call crosses the
// generic relay and this test is RED.
func TestLeafBWaveRoutesAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, entry := range leafbWave {
		if entry.Serve == leafbServeLanded {
			continue
		}
		if entry.Path == "/api/v0/docs" || entry.Path == "/api/v0/redoc" {
			// Static shells file no dispatch row (no dispatch step
			// runs): their ahead-of-relay proof is the no-origin 200
			// with the shell bytes in TestLeafBPublicDocUI, which the
			// generic relay answers 503.
			continue
		}
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			t.Parallel()

			// The two terminated login-picker reads synthesize from the
			// fetched JSON instead of relaying bytes, so the stub answers
			// a body each fetch decodes: the policy fetch reads
			// require_sso, the posture fetch reads the posture keys.
			body := `{"data":[],"error":null,"truth":null}`
			if entry.Path == "/api/v0/auth/sign-in-policy" || entry.Path == "/api/v0/auth/providers" {
				body = `{"require_sso":false,"providers":[],"local_login_offered":true,"self_service_tokens_offered":true}`
			}
			core := &stubCore{status: http.StatusOK, body: body}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			key := entry.Method + " " + entry.Path
			target, ok := leafbConcretePaths[key]
			if !ok {
				t.Fatalf("no concrete path for wave route %q", key)
			}
			method := entry.Method
			req := authedRequest(method, target)
			if method == http.MethodPost {
				req.Body = io.NopCloser(http.NoBody)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(target)
			lanes, ok := snap.dispatches[family]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", family)
			}
			if lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", family, lanes)
			}
			if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1", family, got)
			}
		})
	}
}

// TestLeafBWaveRelayExplicitForUnresolvableIdentities pins the D-wave
// ruling's never-invent rule: a wave-gated route carrying a credential
// class dest cannot resolve (a scoped token, an OIDC bearer, a browser
// session cookie) must relay explicitly to the core — the core judges,
// core bytes come back untouched — and must never answer a
// dest-invented 401, 403, 404, or 500. The lane files on viaRelay so a
// reader can tell core-served from dest-terminated. Today the key gate
// 401s these callers with zero core contact, so this test is RED.
func TestLeafBWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	gated := 0
	for _, entry := range leafbWave {
		if entry.Serve != leafbServeGated {
			continue
		}
		gated++
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			t.Parallel()

			for _, tc := range []struct {
				name   string
				header map[string]string
			}{
				{"scoped bearer", map[string]string{"Authorization": "Bearer scoped-token-abc"}},
				{"oidc bearer", map[string]string{"Authorization": "Bearer oidc-jwt.def.ghi"}},
				{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
				{"browser session (__Host-eshu_session)", map[string]string{"Cookie": "__Host-eshu_session=sess_123"}},
			} {
				core := &stubCore{
					status: http.StatusOK,
					body:   `{"data":[],"error":null,"truth":null}`,
				}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				key := entry.Method + " " + entry.Path
				req := httptest.NewRequest(entry.Method, leafbConcretePaths[key], nil)
				for k, v := range tc.header {
					req.Header.Set(k, v)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Errorf("%s: upstream hits = %d, want 1 (relay explicitly, never gate-invent)", tc.name, core.hits)
				}
				if rec.Code != http.StatusOK {
					t.Errorf("%s: status = %d, want the relayed core 200 (never 401/403/404/500-invented)", tc.name, rec.Code)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != core.body {
					t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, body, core.body)
				}
				family := classifyFamily(leafbConcretePaths[key])
				if got := tel.snapshot().dispatches[family][viaRelay][outcomeCoreOK]; got != 1 {
					t.Errorf("%s: family %q relay core_ok = %d, want 1", tc.name, family, got)
				}
			}
		})
	}
	if gated != 12 {
		t.Fatalf("gated wave routes = %d, want 12 (the D2 termination set)", gated)
	}
}
