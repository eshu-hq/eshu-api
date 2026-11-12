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

// W8a satellite termination latency evidence (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#100). The terminated reader does the
// same single server-side fetch as the generic relay plus the wave
// gate and the family authorizer; these benchmarks bound that
// overhead per wave family so scripts/measure-w8a-latency.sh can
// gate it in CI. The stub core answers from memory — the numbers
// isolate dest-side cost, not core or network time. Absolute
// deployment numbers ride the pinned-core-image leg at flip prep.

// w8aBenchServer builds a Handler over a stub core answering body
// for every route, so terminated and relay benchmarks share the same
// upstream shape.
func w8aBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// w8aBenchPost builds one shared-key wave POST, rebuilt per
// iteration since the server consumes it.
func w8aBenchPost(path string) *http.Request {
	post := authedRequest(http.MethodPost, path)
	body := w8aPostBodies[path]
	post.Body = io.NopCloser(strings.NewReader(body))
	post.ContentLength = int64(len(body))
	return post
}

// BenchmarkW8aTerminatedRead measures the terminated path for a
// shared-key call through the full production chain: one leg per
// wave family through the same wave gate and family authorizer.
func BenchmarkW8aTerminatedRead(b *testing.B) {
	_, srv := w8aBenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	gets := map[string]*http.Request{
		"secrets-iam": authedRequest(http.MethodGet, "/api/v0/secrets-iam/posture-summary"),
		"status":      authedRequest(http.MethodGet, "/api/v0/status/governance"),
		"iac-get":     authedRequest(http.MethodGet, "/api/v0/replatforming/selectors?limit=50"),
		"metrics":     authedRequest(http.MethodGet, "/api/v0/metrics/timeseries?metric=cpu&range=1h"),
	}
	for name, get := range gets {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, get.Clone(get.Context()))
			}
		})
	}
	for name, path := range map[string]string{
		"iac-post":  "/api/v0/replatforming/rollups",
		"semantic":  "/api/v0/search/semantic",
		"terraform": "/api/v0/terraform/config-state-drift/findings",
		"aws-drift": "/api/v0/aws/runtime-drift/findings",
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, w8aBenchPost(path))
			}
		})
	}
}

// BenchmarkW8aRelayRead measures the generic relay path for the same
// caller and stub shape: the before legs the lane must not regress.
// The legs compare different routes by necessity (the W8a paths now
// terminate — the relay GET is the D4-pinned relay story leg and the
// relay POST is the D3/D4-pinned relay bundle leg); every leg does
// exactly one server-side fetch, so the 1.50 bound stays a real
// pathology check.
func BenchmarkW8aRelayRead(b *testing.B) {
	_, srv := w8aBenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	get := authedRequest(http.MethodGet, "/api/v0/dependencies")

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
			post := authedRequest(http.MethodPost, "/api/v0/code/bundles")
			post.Body = io.NopCloser(strings.NewReader(`{"scope_id":"scope_1"}`))
			post.ContentLength = int64(len(`{"scope_id":"scope_1"}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})
}
