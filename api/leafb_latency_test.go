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

// D2 Leaf B latency evidence (Refs eshu-hq/eshu#9669, part of
// #6707). The terminated reader does the same single server-side
// fetch as the generic relay plus the wave gate and the family
// authorizer; these benchmarks bound that overhead per representative
// route so scripts/measure-leafb-latency.sh can gate it in CI. The
// stub core answers from memory — the numbers isolate dest-side cost,
// not core or network time. Absolute deployment numbers ride the
// pinned-core-image leg at flip prep (D11).

// leafbBenchServer builds a Handler over a stub core answering body
// for every route, so terminated and relay benchmarks share the same
// upstream shape.
func leafbBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkLeafBTerminatedRead measures the terminated path for a
// shared-key GET and POST through the full production chain.
func BenchmarkLeafBTerminatedRead(b *testing.B) {
	_, srv := leafbBenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	get := authedRequest(http.MethodGet, "/api/v0/capabilities")
	post := authedRequest(http.MethodPost, "/api/v0/ask")
	post.Body = io.NopCloser(strings.NewReader(`{"question":"q"}`))

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
			req.Body = io.NopCloser(strings.NewReader(`{"question":"q"}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}

// BenchmarkLeafBRelayRead measures the explicit relay path for the
// same stub shape: the before leg the lane must not regress. The
// GET relay leg rides /api/v0/dependencies and the POST relay leg
// rides /api/v0/impact/change-surface, both via foreign-bearer
// explicit relay on their wave-mounted routes (both legs have since
// terminated natively for the shared key — dependencies in
// W8b-narrow, change-surface in the impact lane's Wave-3 — so the
// shared-key shape would run terminated-vs-terminated).
// leafbRelayGet and leafbRelayPost build the re-pointed relay legs shared
// by the benchmark and the StaysRelayed guard: foreign-bearer legs on their
// wave-mounted routes. Both must use these constructors so deleting a bearer
// breaks the guard instead of silently restoring terminated-vs-terminated.
func leafbRelayGet(bearer string) *http.Request {
	get := authedRequest(http.MethodGet, "/api/v0/dependencies")
	get.Header.Set("Authorization", "Bearer "+bearer)
	return get
}

func leafbRelayPost(bearer string) *http.Request {
	post := authedRequest(http.MethodPost, "/api/v0/impact/change-surface")
	post.Header.Set("Authorization", "Bearer "+bearer)
	post.Body = io.NopCloser(strings.NewReader(`{"question":"q"}`))
	return post
}

func BenchmarkLeafBRelayRead(b *testing.B) {
	_, srv := leafbBenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	get := leafbRelayGet("foreign")
	post := leafbRelayPost("foreign")

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
			req.Body = io.NopCloser(strings.NewReader(`{"question":"q"}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}
	})
}
