// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// iacNativeCases is the W3 IaC-6 termination slice cleared on
// IaCReadAdmission v0 (eshu-hq/eshu#10455): the four POST legs whose
// every read the pin adopts. GET /iac/resources and POST /iac/dead
// stay on the generic relay until their cleared coverage lands
// (supplemental SearchActive mint, dual-pin dead disposition).
var iacNativeCases = []struct {
	name    string
	route   string
	payload string
}{
	{"unmanaged-resources", iacUnmanagedResourcesRoute, `{"limit":50}`},
	{"management-status", iacManagementStatusRoute, `{"repositories":["repository:r1"],"limit":50}`},
	{"management-status-explain", iacManagementStatusExplainRoute, `{"repositories":["repository:r1"],"limit":1}`},
	{"terraform-import-plan-candidates", iacTerraformImportPlanCandidatesRoute, `{"repositories":["repository:r1"],"limit":50}`},
}

// iacRoutePath strips the "METHOD " prefix from a native route const.
func iacRoutePath(route string) string {
	_, path, _ := strings.Cut(route, " ")
	return path
}

// TestIaCNative_UnauthenticatedIs401 pins that every native IaC spelling
// keeps the dest 401 shape: envelope callers get the envelope error,
// everyone else the plain error_code body, both with the bare Bearer
// challenge — and the core is never reached.
func TestIaCNative_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range iacNativeCases {
		for _, spelling := range []string{iacRoutePath(tc.route), iacRoutePath(tc.route) + "/"} {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				req := httptest.NewRequest(http.MethodPost, spelling, strings.NewReader(tc.payload))
				if accept != "" {
					req.Header.Set("Accept", accept)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusUnauthorized {
					t.Errorf("%s %q Accept %q: status = %d, want 401", tc.name, spelling, accept, rec.Code)
				}
				if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
					t.Errorf("%s %q Accept %q: WWW-Authenticate = %q, want bare Bearer", tc.name, spelling, accept, got)
				}
			}
		}
	}
}

// TestIaCNative_UnconfiguredOriginFailsClosed pins the fail-closed origin:
// with no core origin configured every native IaC spelling answers the
// permanent 503 backend_unavailable instead of panicking, in both
// negotiated shapes.
func TestIaCNative_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range iacNativeCases {
		for _, spelling := range []string{iacRoutePath(tc.route), iacRoutePath(tc.route) + "/"} {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				req := authedRequest(http.MethodPost, spelling)
				req.Header.Set("Accept", accept)
				req.Body = io.NopCloser(strings.NewReader(tc.payload))
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusServiceUnavailable {
					t.Errorf("%s %q Accept %q: status = %d, want 503", tc.name, spelling, accept, rec.Code)
				}
				if got := rec.Header().Get("Retry-After"); got != "" {
					t.Errorf("%s %q Accept %q: Retry-After = %q, want none (local 503 is permanent)", tc.name, spelling, accept, got)
				}
			}
		}
	}
}

// TestIaCNative_RelaysByteForByte pins the termination parity: the core's
// status, headers, and body cross the native route unmodified — the 200
// truth page, the scoped-grant 403 (dest adds no gate; the core judges),
// each route's fixed 500 message in its exact core bytes, the retryable
// 503 keeping the core's Retry-After, and the keyset paging edge.
func TestIaCNative_RelaysByteForByte(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		status  int
		header  http.Header
		body    string
		payload string
	}{
		{
			"truth page",
			http.StatusOK, nil,
			"{\"data\":{\"total\":2,\"truncated\":false},\"error\":null}\n",
			`{"limit":2}`,
		},
		{
			"grant denial relays untouched",
			http.StatusForbidden, nil,
			"{\"detail\":\"all-scopes authorization is required\",\"error\":\"Forbidden\"}\n",
			`{"limit":50}`,
		},
		{
			"unmanaged fixed 500",
			http.StatusInternalServerError, nil,
			"{\"detail\":\"list unmanaged cloud resources failed\",\"error\":\"Internal Server Error\"}\n",
			`{"limit":50}`,
		},
		{
			"management-status fixed 500",
			http.StatusInternalServerError, nil,
			"{\"detail\":\"IaC management status read failed\",\"error\":\"Internal Server Error\"}\n",
			`{"repositories":["repository:r1"],"limit":50}`,
		},
		{
			"management-explain fixed 500",
			http.StatusInternalServerError, nil,
			"{\"detail\":\"IaC management explanation read failed\",\"error\":\"Internal Server Error\"}\n",
			`{"repositories":["repository:r1"],"limit":1}`,
		},
		{
			"import-plan fixed 500",
			http.StatusInternalServerError, nil,
			"{\"detail\":\"list Terraform import-plan findings failed\",\"error\":\"Internal Server Error\"}\n",
			`{"repositories":["repository:r1"],"limit":50}`,
		},
		{
			"retryable 503 keeps Retry-After",
			http.StatusServiceUnavailable,
			http.Header{"Retry-After": []string{"5"}},
			"{\"detail\":\"IaC inventory database is unavailable\",\"error\":\"Service Unavailable\"}\n",
			`{"limit":50}`,
		},
		{
			"keyset page relays cursor",
			http.StatusOK, nil,
			"{\"data\":{\"total\":201,\"next_cursor\":\"aws_s3_bucket.logs|content-entity:e_9\",\"truncated\":true},\"error\":null}\n",
			`{"limit":11}`,
		},
	}
	for _, tc := range iacNativeCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, relay := range cases {
				for _, accept := range []string{EnvelopeMIME, "application/json"} {
					core := &stubCore{status: relay.status, header: relay.header, body: relay.body}
					mux := http.NewServeMux()
					mux.Handle("/", core.handler())
					upstream := httptest.NewServer(mux)
					defer upstream.Close()

					srv := NewServer(upstream.URL, skeletonTestKey)
					handler := srv.Handler(DefaultV0Sunset)

					target := iacRoutePath(tc.route)
					req := authedRequest(http.MethodPost, target)
					req.Header.Set("Accept", accept)
					req.Body = io.NopCloser(strings.NewReader(relay.payload))
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, req)

					if core.hits != 1 {
						t.Fatalf("%s Accept %q: upstream hits = %d, want exactly 1 (no retry on write-through)", relay.name, accept, core.hits)
					}
					if core.lastMethod != http.MethodPost {
						t.Errorf("%s Accept %q: upstream method = %q, want POST", relay.name, accept, core.lastMethod)
					}
					if core.lastPath != target {
						t.Errorf("%s Accept %q: upstream path = %q, want the core route %q", relay.name, accept, core.lastPath, target)
					}
					if core.lastBody != relay.payload {
						t.Errorf("%s Accept %q: upstream body = %q, want the caller bytes", relay.name, accept, core.lastBody)
					}
					if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
						t.Errorf("%s Accept %q: upstream Authorization = %q, want the forwarded Bearer key", relay.name, accept, core.lastAuth)
					}
					if rec.Code != relay.status {
						t.Errorf("%s Accept %q: status = %d, want relayed %d", relay.name, accept, rec.Code, relay.status)
					}
					if relay.header != nil {
						if got := rec.Header().Get("Retry-After"); got != relay.header.Get("Retry-After") {
							t.Errorf("%s Accept %q: Retry-After = %q, want the core's value relayed", relay.name, accept, got)
						}
					}
					body, _ := io.ReadAll(rec.Result().Body)
					if string(body) != relay.body {
						t.Errorf("%s Accept %q: body = %q, want the core bytes %q", relay.name, accept, body, relay.body)
					}
				}
			}
		})
	}
}

// TestIaCNative_TrailingSlashStaysNative pins the never-split rule: each
// trailing-slash spelling forwards once to the same core route with the
// spelled path, so the family is never split across dispatch paths.
func TestIaCNative_TrailingSlashStaysNative(t *testing.T) {
	t.Parallel()

	for _, tc := range iacNativeCases {
		core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		spelling := iacRoutePath(tc.route) + "/"
		req := authedRequest(http.MethodPost, spelling)
		req.Body = io.NopCloser(strings.NewReader(tc.payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != spelling {
			t.Errorf("%s: hits = %d lastPath = %q, want the native route to forward as spelled", tc.name, core.hits, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", tc.name, rec.Code)
		}
	}
}

// TestIaCNative_MatchesGenericRelay pins the before/after parity at the
// dest layer: the same stub-core answer crosses the native route and the
// generic relay (POST /api/v0/iac/dead, the partial leg that stays on the
// relay) with byte-identical status and body.
func TestIaCNative_MatchesGenericRelay(t *testing.T) {
	t.Parallel()

	const relayPath = "/api/v0/iac/dead"
	bodies := []struct {
		name   string
		status int
		body   string
	}{
		{"truth page", http.StatusOK, "{\"data\":{\"total\":1},\"error\":null}\n"},
		{"fixed 500", http.StatusInternalServerError, "{\"detail\":\"dead IaC scan failed\",\"error\":\"Internal Server Error\"}\n"},
	}
	for _, tc := range iacNativeCases {
		for _, fixture := range bodies {
			core := &stubCore{status: fixture.status, body: fixture.body}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			serve := func(target string) (int, string) {
				req := authedRequest(http.MethodPost, target)
				req.Body = io.NopCloser(strings.NewReader(tc.payload))
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				body, _ := io.ReadAll(rec.Result().Body)
				return rec.Code, string(body)
			}
			nativeCode, nativeBody := serve(iacRoutePath(tc.route))
			relayCode, relayBody := serve(relayPath)

			if nativeCode != relayCode || nativeBody != relayBody {
				t.Errorf("%s %s: native = (%d, %q), relay = (%d, %q): want byte-identical", tc.name, fixture.name, nativeCode, nativeBody, relayCode, relayBody)
			}
		}
	}
}
