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

// Code-search latency evidence (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#113). The terminated reader does the same single
// server-side fetch as the generic relay plus the wave gate and the
// family authorizer; these benchmarks bound that overhead for the two
// code search POSTs. The stub core answers from memory — the numbers
// isolate dest-side cost, not core or network time. Absolute
// deployment numbers ride the pinned-core-image leg at flip prep
// (D11).

// codeSearchBenchServer builds a Handler over a stub core answering
// body for every route, so terminated and relay benchmarks share the
// same upstream shape.
func codeSearchBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// codeSearchBenchPost builds one shared-key code search POST with a
// representative body, rebuilt per iteration since the server consumes
// it.
func codeSearchBenchPost(path string) *http.Request {
	body := codeSearchParityBodies[path][0]
	post := authedRequest(http.MethodPost, path)
	post.Body = io.NopCloser(strings.NewReader(body))
	post.ContentLength = int64(len(body))
	return post
}

// BenchmarkCodeSearchTerminatedPOST measures the terminated path for
// shared-key code search POSTs through the full production chain: one
// leg per wave route through the same wave gate and family authorizer.
func BenchmarkCodeSearchTerminatedPOST(b *testing.B) {
	_, srv := codeSearchBenchServer(b, `{"results":[],"count":0}`)
	handler := srv.Handler(DefaultV0Sunset)

	legs := []string{
		"/api/v0/code/search",
		"/api/v0/code/symbols/search",
	}
	names := map[string]string{
		"/api/v0/code/search":         "code-search",
		"/api/v0/code/symbols/search": "symbols-search",
	}
	for _, path := range legs {
		b.Run(names[path], func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, codeSearchBenchPost(path))
			}
		})
	}
}

// BenchmarkCodeSearchRelayPOST measures the generic relay path for the
// same caller shape: a still-relayed code POST, which crosses the
// generic proxy. The relay leg rides a route no wave table covers
// (POST /api/v0/code/bundles); both do exactly one server-side fetch,
// so the 1.50 bound stays a real pathology check.
func BenchmarkCodeSearchRelayPOST(b *testing.B) {
	_, srv := codeSearchBenchServer(b, `{"results":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("bundles-relay", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			post := authedRequest(http.MethodPost, "/api/v0/code/bundles")
			post.Body = io.NopCloser(strings.NewReader(`{"query":"render"}`))
			post.ContentLength = int64(len(`{"query":"render"}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})
}
