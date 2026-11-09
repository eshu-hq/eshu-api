// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// GitHub-pair termination latency benchmarks (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72). Each leg drives the terminated
// exact path through the full production chain against the same stub
// core the D9 benchmarks use; the relay legs are the pinned generic
// relay shapes. scripts/measure-github-termination-latency.sh compares
// the medians with the 1.50 pathology bound.

// BenchmarkGitHubTermLogin measures the terminated login-start GET:
// bare and with the full provider-IDs-plus-return_to query.
func BenchmarkGitHubTermLogin(b *testing.B) {
	_, srv := d9BenchServer(b, ``)
	handler := srv.Handler(DefaultV0Sunset)

	bare := httptest.NewRequest(http.MethodGet, "/api/v0/auth/github/login", nil)
	full := httptest.NewRequest(http.MethodGet, "/api/v0/auth/github/login?provider_config_id=github-dev&tenant_id=tenant_a&workspace_id=workspace_a&return_to=%2Fconsole", nil)

	b.Run("bare", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, bare.Clone(bare.Context()))
		}
	})
	b.Run("query", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, full.Clone(full.Context()))
		}
	})
}

// BenchmarkGitHubTermCallback measures the terminated callback GET
// carrying the credential state + code.
func BenchmarkGitHubTermCallback(b *testing.B) {
	_, srv := d9BenchServer(b, `{"csrf_token":"opaque-csrf"}`)
	handler := srv.Handler(DefaultV0Sunset)

	callback := httptest.NewRequest(http.MethodGet, "/api/v0/auth/github/callback?state=st-opaque-123&code=cd-opaque-456", nil)

	b.Run("callback", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, callback.Clone(callback.Context()))
		}
	})
}
