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

// Code-graph latency evidence (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#121). The terminated reader does the same single
// server-side fetch as the generic relay plus the wave gate and the
// family authorizer; these benchmarks bound that overhead for the
// four G-a POSTs. The stub core answers from memory — the numbers
// isolate dest-side cost, not core or network time. Absolute
// deployment numbers ride the pinned-core-image leg at flip prep
// (D11).

// codeGraphBenchServer builds a Handler over a stub core answering
// body for every route, so terminated and relay benchmarks share the
// same upstream shape.
func codeGraphBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// codeGraphBenchPost builds one shared-key G-a POST with a
// representative body, rebuilt per iteration since the server consumes
// it.
func codeGraphBenchPost(path, body string) *http.Request {
	post := authedRequest(http.MethodPost, path)
	post.Body = io.NopCloser(strings.NewReader(body))
	post.ContentLength = int64(len(body))
	return post
}

// BenchmarkCodeGraphTerminatedRead measures the terminated path for
// shared-key graph calls through the full production chain: one leg
// per wave route through the same wave gate and family authorizer.
func BenchmarkCodeGraphTerminatedRead(b *testing.B) {
	_, srv := codeGraphBenchServer(b, `{"chains":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("call-chain-post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeGraphBenchPost(codeGraphCallChainPath, codeGraphCallChainBodies[0]))
		}
	})
	b.Run("compare-post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeGraphBenchPost(codeGraphComparePath, codeGraphCompareBodies[0]))
		}
	})
	b.Run("metrics-post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeGraphBenchPost(codeGraphMetricsPath, codeGraphMetricsBodies[0]))
		}
	})
	b.Run("callers-post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeGraphBenchPost(codeGraphRouteCallers, codeGraphCallersBodies[0]))
		}
	})
}

// BenchmarkCodeGraphRelayRead measures the generic relay path for the
// same caller shape: a still-relayed POST on the generic proxy, which
// does exactly one server-side fetch, so the 1.50 bound stays a real
// pathology check. The relay leg rides a route no wave table covers
// (POST /api/v0/code/bundles).
func BenchmarkCodeGraphRelayRead(b *testing.B) {
	_, srv := codeGraphBenchServer(b, `{"chains":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("post-relay", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			post := authedRequest(http.MethodPost, "/api/v0/code/bundles")
			post.Body = io.NopCloser(strings.NewReader(`{"query":"q"}`))
			post.ContentLength = int64(len(`{"query":"q"}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})
}
