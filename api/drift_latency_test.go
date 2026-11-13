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

// Drift latency evidence (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#117). The terminated reader does the same single
// server-side fetch as the generic relay plus the wave gate and the
// family authorizer; these benchmarks bound that overhead for the
// findings POST and the packet GET. The stub core answers from
// memory — the numbers isolate dest-side cost, not core or network
// time. Absolute deployment numbers ride the pinned-core-image leg at
// flip prep (D11).

// driftBenchServer builds a Handler over a stub core answering body
// for every route, so terminated and relay benchmarks share the same
// upstream shape.
func driftBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// driftBenchPost builds one shared-key drift findings POST with a
// representative body, rebuilt per iteration since the server consumes
// it.
func driftBenchPost() *http.Request {
	body := driftFindingsBodies[0]
	post := authedRequest(http.MethodPost, driftFindingsPath)
	post.Body = io.NopCloser(strings.NewReader(body))
	post.ContentLength = int64(len(body))
	return post
}

// driftBenchGet builds one shared-key drift packet GET with a
// representative query, rebuilt per iteration.
func driftBenchGet() *http.Request {
	return authedRequest(http.MethodGet, driftPacketTargets[0])
}

// BenchmarkDriftTerminatedRead measures the terminated path for
// shared-key drift calls through the full production chain: one leg
// per wave route through the same wave gate and family authorizer.
func BenchmarkDriftTerminatedRead(b *testing.B) {
	_, srv := driftBenchServer(b, `{"drift_findings":[],"findings_count":0}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("findings-post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, driftBenchPost())
		}
	})
	b.Run("packet-get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, driftBenchGet())
		}
	})
}

// BenchmarkDriftRelayRead measures the generic relay path for the
// same caller shape: a still-relayed GET and a still-relayed POST,
// which cross the generic proxy. The relay legs ride routes no wave
// table covers (the D4-pinned relay story leg is keyed, so the GET
// rides /api/v0/dependencies and the POST rides
// /api/v0/code/bundles); both do exactly one server-side fetch, so
// the 1.50 bound stays a real pathology check.
func BenchmarkDriftRelayRead(b *testing.B) {
	_, srv := driftBenchServer(b, `{"drift_findings":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("get-relay", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v0/dependencies"))
		}
	})
	b.Run("post-relay", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			post := authedRequest(http.MethodPost, "/api/v0/code/bundles")
			post.Body = io.NopCloser(strings.NewReader(`{"scope_id":"scope_1"}`))
			post.ContentLength = int64(len(`{"scope_id":"scope_1"}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})
}
