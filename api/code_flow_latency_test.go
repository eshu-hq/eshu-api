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

// Code-flow relay-leg latency evidence (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#98, W6a). The terminated reader does
// the same single server-side fetch as the generic relay plus the
// wave gate and the family authorizer; these benchmarks bound that
// overhead for the four Code-flow POSTs. The stub core answers from
// memory — the numbers isolate dest-side cost, not core or network
// time. Absolute deployment numbers ride the pinned-core-image leg at
// flip prep.

// codeFlowBenchServer builds a Handler over a stub core answering
// body for a Code-flow route, so the terminated and relay benchmarks
// share the same upstream shape.
func codeFlowBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// codeFlowBenchPost builds one shared-key Code-flow POST with the
// parity body, rebuilt per iteration since the server consumes it.
func codeFlowBenchPost(path string) *http.Request {
	post := authedRequest(http.MethodPost, path)
	post.Body = io.NopCloser(strings.NewReader(codeFlowParityBody))
	post.ContentLength = int64(len(codeFlowParityBody))
	return post
}

// BenchmarkCodeFlowTerminatedPOST measures the terminated path for
// shared-key Code-flow POSTs through the full production chain: the
// wave gate plus the family authorizer plus the single fetch.
func BenchmarkCodeFlowTerminatedPOST(b *testing.B) {
	_, srv := codeFlowBenchServer(b, `{"data":[],"truth":{"coverage":"exact"}}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("cfg-summary", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			post := codeFlowBenchPost("/api/v0/code/flow/cfg-summary")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})

	b.Run("taint-path", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			post := codeFlowBenchPost("/api/v0/code/flow/taint-path")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})

	b.Run("pdg-summary", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			post := codeFlowBenchPost("/api/v0/code/flow/pdg-summary")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})

	b.Run("reaching-def", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			post := codeFlowBenchPost("/api/v0/code/flow/reaching-def")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})
}

// BenchmarkCodeFlowRelayPOST measures the generic relay path for the
// same caller shape: a still-relayed code POST, which crosses the
// generic proxy. Both legs do exactly one server-side fetch, so the
// comparison stays a real pathology check.
func BenchmarkCodeFlowRelayPOST(b *testing.B) {
	_, srv := codeFlowBenchServer(b, `{"data":[],"truth":{"coverage":"exact"}}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("bundles-relay", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			post := authedRequest(http.MethodPost, "/api/v0/code/bundles")
			post.Body = io.NopCloser(strings.NewReader(codeFlowParityBody))
			post.ContentLength = int64(len(codeFlowParityBody))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})
}
