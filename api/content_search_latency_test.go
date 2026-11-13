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

// W4 Content search latency evidence (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#111). The terminated reader does the same single
// server-side fetch as the generic relay plus the wave gate and the
// family authorizer; these benchmarks bound that overhead for the four
// search POSTs. The stub core answers from memory — the numbers isolate
// dest-side cost, not core or network time. Absolute deployment numbers
// ride the pinned-core-image leg at flip prep (D11).

// contentSearchBenchServer builds a Handler over a stub core answering
// body for every route, so terminated and relay benchmarks share the
// same upstream shape.
func contentSearchBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// contentSearchBenchPost builds one shared-key search POST with a
// representative body, rebuilt per iteration since the server consumes
// it.
func contentSearchBenchPost(path string) *http.Request {
	body := contentSearchParityBodies[path][0]
	post := authedRequest(http.MethodPost, path)
	post.Body = io.NopCloser(strings.NewReader(body))
	post.ContentLength = int64(len(body))
	return post
}

// BenchmarkContentSearchTerminatedPOST measures the terminated path
// for shared-key search POSTs through the full production chain: one
// leg per wave route through the same wave gate and family authorizer.
func BenchmarkContentSearchTerminatedPOST(b *testing.B) {
	_, srv := contentSearchBenchServer(b, `{"results":[],"count":0}`)
	handler := srv.Handler(DefaultV0Sunset)

	legs := []string{
		"/api/v0/content/files/search",
		"/api/v0/content/entities/search",
		"/api/v0/content/dockerfile-installs/search",
		"/api/v0/content/php-config/search",
	}
	names := map[string]string{
		"/api/v0/content/files/search":               "files-search",
		"/api/v0/content/entities/search":            "entities-search",
		"/api/v0/content/dockerfile-installs/search": "dockerfile-search",
		"/api/v0/content/php-config/search":          "php-config-search",
	}
	for _, path := range legs {
		b.Run(names[path], func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, contentSearchBenchPost(path))
			}
		})
	}
}

// BenchmarkContentSearchRelayPOST measures the generic relay path for
// the same caller shape: a still-relayed code POST, which crosses the
// generic proxy. The search legs now terminate, so the relay leg rides
// a route no wave table covers; both do exactly one server-side fetch,
// so the 1.50 bound stays a real pathology check.
func BenchmarkContentSearchRelayPOST(b *testing.B) {
	_, srv := contentSearchBenchServer(b, `{"results":[]}`)
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
