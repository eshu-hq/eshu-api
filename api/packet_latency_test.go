// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Supply-chain impact packet latency evidence (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#97). The terminated reader does the same
// single server-side fetch as the generic relay plus the wave gate and
// the family authorizer; this benchmark bounds that overhead for the
// packet single. The stub core answers from memory — the number
// isolates dest-side cost, not core or network time. Absolute
// deployment numbers ride the pinned-core-image leg at flip prep
// (D11).

// BenchmarkPacketTerminatedRead measures the terminated path for a
// shared-key packet GET through the full production chain: the wave
// gate plus the family authorizer plus the single core fetch.
func BenchmarkPacketTerminatedRead(b *testing.B) {
	core := &stubCore{status: http.StatusOK, body: `{"packet":{},"truth":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("packet-get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, authedRequest(http.MethodGet, packetConcreteTarget))
		}
	})
}
