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

// Code-divergence latency evidence (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#125). The terminated reader does the same single
// server-side fetch as the generic relay plus the wave gate and the
// family authorizer; these benchmarks bound that overhead for the
// three divergence POSTs. The stub core answers from memory — the
// numbers isolate dest-side cost, not core or network time. Absolute
// deployment numbers ride the pinned-core-image leg at flip prep
// (D11).

// codeDivergenceBenchServer builds a Handler over a stub core
// answering body for every route, so terminated and relay benchmarks
// share the same upstream shape.
func codeDivergenceBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// codeDivergenceBenchPost builds one shared-key divergence POST with
// a representative body, rebuilt per iteration since the server
// consumes it.
func codeDivergenceBenchPost(path, body string) *http.Request {
	post := authedRequest(http.MethodPost, path)
	post.Body = io.NopCloser(strings.NewReader(body))
	post.ContentLength = int64(len(body))
	return post
}

// BenchmarkCodeDivergenceTerminatedRead measures the terminated path
// for shared-key divergence calls through the full production chain:
// one leg per wave route through the same wave gate and family
// authorizer.
func BenchmarkCodeDivergenceTerminatedRead(b *testing.B) {
	_, srv := codeDivergenceBenchServer(b, `{"findings":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("findings-post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeDivergenceBenchPost(codeDivergenceFindingsPath, codeDivergenceFindingsBodies[0]))
		}
	})
	b.Run("investigate-post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeDivergenceBenchPost(codeDivergenceInvestigatePath, codeDivergenceInvestigateBodies[0]))
		}
	})
	b.Run("report-post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeDivergenceBenchPost(codeDivergenceReportPath, codeDivergenceReportBodies[0]))
		}
	})
}

// BenchmarkCodeDivergenceRelayRead measures the generic relay path
// for the same caller shape: a still-relayed POST on the generic
// proxy, which does exactly one server-side fetch, so the 1.50 bound
// stays a real pathology check. The relay leg rides a route no wave
// table covers (POST /api/v0/code/bundles).
func BenchmarkCodeDivergenceRelayRead(b *testing.B) {
	_, srv := codeDivergenceBenchServer(b, `{"findings":[]}`)
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
