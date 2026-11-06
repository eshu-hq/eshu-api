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

// D3 terminated-vs-relay byte parity (Refs eshu-hq/eshu#9669, part of
// #6707). The terminated reader must not distort the core's answer:
// for every stub-core outcome its client-visible bytes must equal the
// generic byte-relay's. This guards the dest-owned response mapping
// (forwardToCore on the native lane) against future divergence from
// the relay contract it mirrors.

// d3ParityRoutes covers one concrete path per D3 family for the parity
// matrix. POST families carry a representative payload; the matrix
// also drives query strings on the GET families.
var d3ParityRoutes = []struct {
	method  string
	path    string
	payload string
}{
	{http.MethodPost, "/api/v0/admin/work-items/query", `{"limit":10,"timeout_ms":5000}`},
	{http.MethodPost, "/api/v0/admin/decisions/query", `{"limit":10}`},
	{http.MethodGet, "/api/v0/auth/admin/roles", ""},
	{http.MethodGet, "/api/v0/auth/local/invitations", ""},
	{http.MethodGet, "/api/v0/auth/admin/audit/events?limit=5", ""},
	{http.MethodGet, "/api/v0/auth/admin/audit/summary", ""},
	{http.MethodPost, "/api/v0/admin/collector-failures/query", `{"collector_kind":"k","instance_id":"i"}`},
	{http.MethodPost, "/api/v0/admin/collector-failures/acknowledgements/query", ""},
}

// TestD3TerminatedMatchesRelayByteForByte pins that the terminated
// reader relays core bytes exactly like the generic proxy: same
// status, same headers (modulo per-hop framing), same body — across
// the pinned outcome matrix in both negotiated shapes. The matrix runs
// every D3 family route; the stub answers the pinned wire strings
// where core owns the text, and opaque bytes where the relay (not the
// text) is what this test pins.
func TestD3TerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	outcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"plain 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"data":[],"error":null,"truth":null}`},
		{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"limit is required"}`},
		{"core all-scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`},
		{"tenant scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"admin tenant scope is required"}`},
		{"missing 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"decision not found"}`},
		{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the admin store is not configured"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"the read hit a stale reader"}`},
		{"query timeout 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"dead-letter query timed out"}`},
		{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/api/v0/auth/oidc/login"}}, ""},
	}
	for _, route := range d3ParityRoutes {
		for _, outcome := range outcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				name := route.method + " " + route.path + " " + outcome.name + " " + accept
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					nativeCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
					nativeMux := http.NewServeMux()
					nativeMux.Handle("/", nativeCore.handler())
					nativeUpstream := httptest.NewServer(nativeMux)
					defer nativeUpstream.Close()

					relayCore := &stubCore{status: outcome.status, header: relayHeaders(outcome.header), body: outcome.body}
					relayMux := http.NewServeMux()
					relayMux.Handle("/", relayCore.handler())
					relayUpstream := httptest.NewServer(relayMux)
					defer relayUpstream.Close()

					build := func(target string) *http.Request {
						req := authedRequest(route.method, target)
						req.Header.Set("Accept", accept)
						req.Header.Set("X-Correlation-ID", "d3-parity-1")
						if route.payload != "" {
							req.Body = io.NopCloser(strings.NewReader(route.payload))
						}
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
					if nativeRes.StatusCode != relayRes.StatusCode {
						t.Fatalf("native status = %d, relay status = %d, want identical", nativeRes.StatusCode, relayRes.StatusCode)
					}
					nativeBody, _ := io.ReadAll(nativeRes.Body)
					relayBody, _ := io.ReadAll(relayRes.Body)
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

// TestD3Terminated_ForwardsCallerBytes pins that the terminated
// reader forwards the caller's method, path, query, negotiation
// headers, credential, and body to the same core route: exactly one
// upstream hit, no dest retry, no credential widening.
func TestD3Terminated_ForwardsCallerBytes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	payload := `{"limit":10,"timeout_ms":5000}`
	req := authedRequest(http.MethodPost, "/api/v0/admin/decisions/query?limit=10")
	req.Header.Set("Accept", EnvelopeMIME)
	req.Body = io.NopCloser(strings.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want exactly 1 (no retry on terminated reads)", core.hits)
	}
	if core.lastMethod != http.MethodPost {
		t.Errorf("upstream method = %q, want POST", core.lastMethod)
	}
	if core.lastPath != "/api/v0/admin/decisions/query" {
		t.Errorf("upstream path = %q, want the core route", core.lastPath)
	}
	if core.lastQuery != "limit=10" {
		t.Errorf("upstream query = %q, want limit=10", core.lastQuery)
	}
	if core.lastBody != payload {
		t.Errorf("upstream body = %q, want the caller bytes", core.lastBody)
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
