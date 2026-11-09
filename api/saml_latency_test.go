// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// SAML-triple termination latency benchmarks (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72). Each leg drives the terminated
// exact path through the full production chain against the same stub
// core the D9 benchmarks use; the relay legs are the pinned generic
// relay shapes. scripts/measure-saml-termination-latency.sh compares
// the medians with the 1.50 pathology bound.

// BenchmarkSAMLTermMetadata measures the terminated metadata GET.
func BenchmarkSAMLTermMetadata(b *testing.B) {
	_, srv := d9SAMLBenchServer(b, `<EntityDescriptor entityID="https://eshu.example/saml/acme-munich"/>`)
	handler := srv.Handler(DefaultV0Sunset)

	metadata := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/metadata", nil)

	b.Run("metadata", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, metadata.Clone(metadata.Context()))
		}
	})
}

// BenchmarkSAMLTermLogin measures the terminated login-start GET:
// bare and with the provider path value plus the realistic return_to
// query.
func BenchmarkSAMLTermLogin(b *testing.B) {
	_, srv := d9SAMLBenchServer(b, ``)
	handler := srv.Handler(DefaultV0Sunset)

	bare := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login", nil)
	full := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", nil)

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

// BenchmarkSAMLTermACS measures the terminated ACS POST carrying the
// representative large form body.
func BenchmarkSAMLTermACS(b *testing.B) {
	_, srv := d9SAMLBenchServer(b, `{"csrf_token":"opaque-csrf"}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("acs", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(d9SAMLACSLargeForm)))
		for i := 0; i < b.N; i++ {
			acs := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(d9SAMLACSLargeForm))
			acs.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, acs)
		}
	})
}
