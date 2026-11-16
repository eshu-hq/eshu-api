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

// Entity latency evidence (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#97). The terminated reader does the same single
// server-side fetch as the generic relay plus the wave gate and the
// family authorizer; these benchmarks bound that overhead for all 9
// terminated legs. The stub core answers from memory
// — the numbers isolate dest-side cost, not core or network time.
// Absolute deployment numbers ride the pinned-core-image leg at flip
// prep (D11).

// entityBenchServer builds a Handler over a stub core answering body
// for every route, so terminated and relay benchmarks share the same
// upstream shape.
func entityBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// entityBenchRequest rebuilds one shared-key entity request per
// iteration, since the server consumes the body.
func entityBenchRequest(leg struct {
	key    string
	method string
	target string
	body   string
},
) *http.Request {
	var req *http.Request
	if leg.method == http.MethodPost {
		req = authedRequest(http.MethodPost, leg.target)
		req.Body = io.NopCloser(strings.NewReader(leg.body))
		req.ContentLength = int64(len(leg.body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = authedRequest(http.MethodGet, leg.target)
	}
	return req
}

// BenchmarkEntityTerminatedRead measures the terminated path for
// shared-key entity calls through the full production chain: one leg
// per wave row through the same wave gate and family authorizer (the
// relay comparators live in BenchmarkImpactRelayRead, which rides the
// explicit relay lane via foreign-bearer callers on the same
// single-fetch shape).
func BenchmarkEntityTerminatedRead(b *testing.B) {
	_, srv := entityBenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	for _, leg := range entityParityLegs {
		b.Run(strings.ReplaceAll(strings.TrimPrefix(leg.target, "/api/v0/"), "/", "-"), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, entityBenchRequest(leg))
			}
		})
	}
}
