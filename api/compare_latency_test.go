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

// Compare relay-leg latency evidence (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72, gap 04). The terminated reader does
// the same single server-side fetch as the generic relay plus the
// wave gate and the family authorizer; these benchmarks bound that
// overhead for the Compare POST so scripts/measure-compare-latency.sh
// can gate it in CI. The stub core answers from memory — the numbers
// isolate dest-side cost, not core or network time. Absolute
// deployment numbers ride the pinned-core-image leg at flip prep
// (D11).

// compareBenchServer builds a Handler over a stub core answering
// body for the Compare route, so the terminated and relay
// benchmarks share the same upstream shape.
func compareBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// compareBenchPost builds one shared-key Compare POST with the
// parity body, rebuilt per iteration since the server consumes it.
func compareBenchPost() *http.Request {
	post := authedRequest(http.MethodPost, compareConcretePath)
	post.Body = io.NopCloser(strings.NewReader(compareParityBody))
	post.ContentLength = int64(len(compareParityBody))
	return post
}

// BenchmarkCompareTerminatedPOST measures the terminated path for a
// shared-key Compare POST through the full production chain: the
// wave gate plus the family authorizer plus the single fetch.
func BenchmarkCompareTerminatedPOST(b *testing.B) {
	_, srv := compareBenchServer(b, `{"workload":"w1","story":"same"}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			post := compareBenchPost()
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})
}

// BenchmarkCompareRelayPOST measures the generic relay path for the
// same caller shape: POST /api/v0/code/bundles, which still crosses
// the generic proxy (the D3/D4/Leaf B lanes pin it as their relay
// POST leg). Both legs do exactly one server-side fetch, so the 1.50
// bound stays a real pathology check.
func BenchmarkCompareRelayPOST(b *testing.B) {
	_, srv := compareBenchServer(b, `{"workload":"w1","story":"same"}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			post := authedRequest(http.MethodPost, "/api/v0/code/bundles")
			post.Body = io.NopCloser(strings.NewReader(compareParityBody))
			post.ContentLength = int64(len(compareParityBody))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})
}
