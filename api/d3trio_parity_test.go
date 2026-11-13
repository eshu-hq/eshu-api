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

// D3 Gap 02 trio terminated-vs-relay byte parity (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#70). The terminated
// reader must not distort the core's answer: for every stub-core
// outcome its client-visible bytes must equal the generic
// byte-relay's. This guards the dest-owned response mapping
// (forwardToCore on the native lane) against future divergence from
// the relay contract it mirrors.

// d3TrioParityRoutes covers one concrete path per trio route. The
// delta-union read carries its required scope filter and the reindex
// progress read its scope_id; the tuning report takes no query. The
// tuning 200 body below is the exact frozen report body
// (SharedProjectionTuningReport v0: status not_applicable plus the
// frozen detail text), so the matrix also proves the native lane
// relays the pinned static shape byte-identical.
var d3TrioParityRoutes = []struct {
	method string
	path   string
	body   string
}{
	{http.MethodGet, "/api/v0/admin/delta-union/completeness?scope=s1", `{"data":[],"error":null,"truth":null}`},
	{http.MethodGet, "/api/v0/admin/reindex/progress?scope_id=s1", `{"data":[],"error":null,"truth":null}`},
	{http.MethodGet, "/api/v0/admin/shared-projection/tuning-report", `{"detail":"Shared-projection tuning is managed internally by the Go projector pipeline.","status":"not_applicable"}`},
}

// TestD3TrioTerminatedMatchesRelayByteForByte pins that the
// terminated trio reader relays core bytes exactly like the generic
// proxy: same status, same headers (modulo per-hop framing), same
// body — across the pinned outcome matrix in both negotiated shapes.
// The matrix runs every trio route; the stub answers the pinned wire
// strings where core owns the text, and opaque bytes where the relay
// (not the text) is what this test pins.
func TestD3TrioTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	outcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"plain 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, ""},
		{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"scope is required"}`},
		{"core all-scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`},
		{"tenant scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"admin tenant scope is required"}`},
		{"missing 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"scope not found"}`},
		{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the admin store is not configured"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"the read hit a stale reader"}`},
		{"query timeout 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"the graph read exceeded its deadline"}`},
		{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/api/v0/auth/oidc/login"}}, ""},
	}
	for _, route := range d3TrioParityRoutes {
		for _, outcome := range outcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				name := route.method + " " + route.path + " " + outcome.name + " " + accept
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					body := outcome.body
					if outcome.name == "plain 200" {
						body = route.body
					}
					nativeCore := &stubCore{status: outcome.status, header: outcome.header, body: body}
					nativeMux := http.NewServeMux()
					nativeMux.Handle("/", nativeCore.handler())
					nativeUpstream := httptest.NewServer(nativeMux)

					relayCore := &stubCore{status: outcome.status, header: relayHeaders(outcome.header), body: body}
					relayMux := http.NewServeMux()
					relayMux.Handle("/", relayCore.handler())
					relayUpstream := httptest.NewServer(relayMux)

					build := func(target string) *http.Request {
						req := authedRequest(route.method, target)
						req.Header.Set("Accept", accept)
						req.Header.Set("X-Correlation-ID", "d3-trio-parity-1")
						return req
					}

					nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
					nativeRec := httptest.NewRecorder()
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, build(route.path))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					// The relay side crosses the same middleware chain
					// the router builds for the generic proxy
					// (instrumented + key gate), so correlation echo,
					// deprecation headers, and lane filing match the
					// production relay path exactly.
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, build(route.path))

					nativeRes, relayRes := nativeRec.Result(), relayRec.Result()
					nativeBody, _ := io.ReadAll(nativeRes.Body)
					relayBody, _ := io.ReadAll(relayRes.Body)
					nativeUpstream.Close()
					relayUpstream.Close()
					if nativeRes.StatusCode != relayRes.StatusCode {
						t.Fatalf("native status = %d, relay status = %d, want identical", nativeRes.StatusCode, relayRes.StatusCode)
					}
					if string(nativeBody) != string(relayBody) {
						t.Errorf("native body = %q, relay body = %q, want identical", nativeBody, relayBody)
					}
					for _, header := range []string{"Content-Type", "Retry-After", "Location", "Cache-Control", "X-Correlation-ID"} {
						if nativeRes.Header.Get(header) != relayRes.Header.Get(header) {
							t.Errorf("header %s: native = %q, relay = %q, want identical",
								header, nativeRes.Header.Get(header), relayRes.Header.Get(header))
						}
					}
					if nativeCore.hits != 1 || relayCore.hits != 1 {
						t.Errorf("upstream hits native = %d relay = %d, want exactly 1 each (no retry on terminated reads)",
							nativeCore.hits, relayCore.hits)
					}
				})
			}
		}
	}
}

// TestD3TrioTerminated_ForwardsCallerBytes pins that the terminated
// trio reader forwards the caller's method, path, query, negotiation
// headers, credential, and body to the same core route: exactly one
// upstream hit, no dest retry, no credential widening.
func TestD3TrioTerminated_ForwardsCallerBytes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v0/admin/delta-union/completeness?scope=s1&scope=s2")
	req.Header.Set("Accept", EnvelopeMIME)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want exactly 1 (no retry on terminated reads)", core.hits)
	}
	if core.lastMethod != http.MethodGet {
		t.Errorf("upstream method = %q, want GET", core.lastMethod)
	}
	if core.lastPath != "/api/v0/admin/delta-union/completeness" {
		t.Errorf("upstream path = %q, want the core route", core.lastPath)
	}
	if core.lastQuery != "scope=s1&scope=s2" {
		t.Errorf("upstream query = %q, want scope=s1&scope=s2", core.lastQuery)
	}
	if core.lastAccept != EnvelopeMIME {
		t.Errorf("upstream Accept = %q, want the caller negotiation", core.lastAccept)
	}
	if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
		t.Errorf("upstream Authorization = %q, want the forwarded Bearer [REDACTED]", core.lastAuth)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != core.body {
		t.Errorf("body = %q, want the core bytes %q", body, core.body)
	}
}
