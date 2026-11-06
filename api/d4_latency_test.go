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

// D4 Native-next latency evidence (Refs eshu-hq/eshu#9669, part of
// #6707). The terminated reader does the same single server-side
// fetch as the generic relay plus the wave gate and the family
// authorizer; these benchmarks bound that overhead per representative
// route so scripts/measure-d4-latency.sh can gate it in CI. The stub
// core answers from memory — the numbers isolate dest-side cost, not
// core or network time. Absolute deployment numbers ride the
// pinned-core-image leg at flip prep (D11).

// d4BenchServer builds a Handler over a stub core answering body
// for every route, so terminated and relay benchmarks share the same
// upstream shape.
func d4BenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkD4TerminatedRead measures the terminated path for a
// shared-key GET and POST through the full production chain.
func BenchmarkD4TerminatedRead(b *testing.B) {
	_, srv := d4BenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	get := authedRequest(http.MethodGet, "/api/v0/repositories")
	post := authedRequest(http.MethodPost, "/api/v0/infra/resources/search")
	post.Body = io.NopCloser(strings.NewReader(`{"scope_id":"s1"}`))

	b.Run("get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, get.Clone(get.Context()))
		}
	})
	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := post.Clone(post.Context())
			req.Body = io.NopCloser(strings.NewReader(`{"scope_id":"s1"}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}

// BenchmarkD4RelayRead measures the generic relay path for the
// same caller and stub shape: the before leg the lane must not
// regress. The legs compare different routes by necessity (the D4
// paths now terminate, and the D3 relay legs moved into this wave);
// both do exactly one server-side fetch, so the 1.50 bound stays a
// real pathology check.
func BenchmarkD4RelayRead(b *testing.B) {
	_, srv := d4BenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	get := authedRequest(http.MethodGet, "/api/v0/dependencies")
	post := authedRequest(http.MethodPost, "/api/v0/code/bundles")
	post.Body = io.NopCloser(strings.NewReader(`{"surface":"x"}`))

	b.Run("get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, get.Clone(get.Context()))
		}
	})
	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := post.Clone(post.Context())
			req.Body = io.NopCloser(strings.NewReader(`{"surface":"x"}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}
