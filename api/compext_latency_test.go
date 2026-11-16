// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Wave-1b latency evidence (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#101). The terminated reader does the same single
// server-side fetch as the generic relay plus the wave gate and the
// family authorizer; these benchmarks bound that overhead for the 2
// component-extension routes. The stub core answers from memory —
// the numbers isolate dest-side cost, not core or network time.
// Absolute deployment numbers ride the pinned-core-image leg at
// flip prep (D11).

// compextBenchServer builds a Handler over a stub core answering
// body for every route, so terminated and relay benchmarks share the
// same upstream shape.
func compextBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// compextBenchReq builds one shared-key component-extension request
// with a representative body, rebuilt per iteration since the server
// consumes it.
func compextBenchReq(method, path, body string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	return req
}

// BenchmarkCompextTerminated measures the terminated path for
// shared-key component-extension reads through the full production
// chain: one leg per wave route through the same wave gate and
// family authorizer.
func BenchmarkCompextTerminated(b *testing.B) {
	_, srv := compextBenchServer(b, `{"items":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	legs := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v0/component-extensions", ""},
		{http.MethodGet, "/api/v0/component-extensions/comp-1/diagnostics", ""},
	}

	for _, leg := range legs {
		b.Run(leg.method+" "+leg.path, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, compextBenchReq(leg.method, leg.path, leg.body))
				if rec.Code != http.StatusOK {
					b.Fatalf("status = %d, want 200", rec.Code)
				}
			}
		})
	}
}

// BenchmarkCompextRelay measures the generic-relay path for the same
// legs, so the terminated overhead stays bounded against the relay
// baseline on the same upstream shape.
func BenchmarkCompextRelay(b *testing.B) {
	_, srv := compextBenchServer(b, `{"items":[]}`)

	legs := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v0/component-extensions", ""},
		{http.MethodGet, "/api/v0/component-extensions/comp-1/diagnostics", ""},
	}

	for _, leg := range legs {
		b.Run(leg.method+" "+leg.path, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				srv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(srv.dispatch))).ServeHTTP(rec, compextBenchReq(leg.method, leg.path, leg.body))
				if rec.Code != http.StatusOK {
					b.Fatalf("status = %d, want 200", rec.Code)
				}
			}
		})
	}
}
