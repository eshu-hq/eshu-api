// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// D7 admin-four latency evidence (Refs eshu-hq/eshu#9669, part of
// #6707). The tier-C clone does the same single server-side fetch as
// the generic relay plus the explicit route match and the family
// authorizer; these benchmarks bound that overhead per representative
// route so scripts/measure-d7-latency.sh can gate it in CI. The stub
// core answers from memory — the numbers isolate dest-side cost, not
// core or network time. Absolute deployment numbers ride the
// pinned-core-image leg at flip prep (D11).

// d7BenchServer builds a Handler over a stub core answering body for
// every route, so clone and relay benchmarks share the same upstream
// shape.
func d7BenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkD7NativeWrite measures the clone path for a shared-key POST
// through the full production chain: one dead-letter leg and one
// reindex leg (the 202 branch, the most expensive entry in the lane).
func BenchmarkD7NativeWrite(b *testing.B) {
	_, srv := d7BenchServer(b, `{"status":"accepted"}`)
	handler := srv.Handler(DefaultV0Sunset)

	deadLetterPost := authedRequest(http.MethodPost, "/api/v0/admin/dead-letter")
	reindexPost := authedRequest(http.MethodPost, "/api/v0/admin/reindex")

	b.Run("dead-letter", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, deadLetterPost.Clone(deadLetterPost.Context()))
		}
	})
	b.Run("reindex", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, reindexPost.Clone(reindexPost.Context()))
		}
	})
}

// BenchmarkD7RelayWrite measures the generic relay path for the same
// caller and stub shape: the before leg the lane must not regress.
// The legs compare different routes by necessity (the D7 paths now
// clone); both do exactly one server-side fetch, so the 1.50 bound
// stays a real pathology check. The relay leg is the Compare POST,
// which stays on the generic relay by D6 disclosure (no pin covers
// its shared graph/content ports).
func BenchmarkD7RelayWrite(b *testing.B) {
	_, srv := d7BenchServer(b, `{"status":"accepted"}`)
	handler := srv.Handler(DefaultV0Sunset)

	post := authedRequest(http.MethodPost, "/api/v0/compare/environments")

	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post.Clone(post.Context()))
		}
	})
}
