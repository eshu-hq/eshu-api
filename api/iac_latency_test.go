// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// W3 IaC-6 latency evidence (Refs eshu-hq/eshu-api#102, part of
// eshu-hq/eshu#9669). The native legs do the same single server-side
// fetch as the generic relay plus the explicit route match and
// per-family telemetry; these benchmarks bound that overhead per leg
// so scripts/measure-iac-latency.sh can gate it in CI. The stub core
// answers from memory — the numbers isolate dest-side cost, not core
// or network time. Absolute deployment numbers ride the
// pinned-core-image leg at flip prep (D11). The dead leg carries a
// second worst-case benchmark: the derived branch analyzes up to
// core's deadFileLimit files per repo, so the gate measures the
// largest page the route can serve.

// iacBenchServer builds a Handler over a stub core answering body for
// every route, so native and relay benchmarks share the same upstream
// shape.
func iacBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkIaCNativeRead measures the five cleared POST legs through
// the full production chain: one shared-key POST per leg.
func BenchmarkIaCNativeRead(b *testing.B) {
	_, srv := iacBenchServer(b, `{"data":{"total":0},"error":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	bench := func(name, path string) {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			req := authedRequest(http.MethodPost, path)
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req.Clone(req.Context()))
			}
		})
	}
	bench("unmanaged-resources", iacRoutePath(iacUnmanagedResourcesRoute))
	bench("management-status", iacRoutePath(iacManagementStatusRoute))
	bench("management-status-explain", iacRoutePath(iacManagementStatusExplainRoute))
	bench("terraform-import-plan-candidates", iacRoutePath(iacTerraformImportPlanCandidatesRoute))
	bench("dead", iacRoutePath(iacDeadRoute))
}

// deadDerivedWorstCaseFiles mirrors core's deadFileLimit
// (go/internal/query/iac/handler.go): the derived branch analyzes up
// to 10,000 files per repo, so the worst-case derived page carries
// that many findings. The stub core answers the page from memory —
// the number isolates dest-side relay cost for the largest page the
// route can serve.
const deadDerivedWorstCaseFiles = 10000

// iacDeadDerivedWorstCaseBody builds one worst-case derived page:
// deadDerivedWorstCaseFiles minimal findings in the core page shape
// (keys alphabetical, as encoding/json marshals the handler map).
func iacDeadDerivedWorstCaseBody() string {
	var sb strings.Builder
	sb.WriteString(`{"data":{"analysis_status":"derived_candidate_analysis","findings":[`)
	for i := 0; i < deadDerivedWorstCaseFiles; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"artifact":"terraform-modules/modules/mod-%d/main.tf","confidence":0.9,"evidence":["module directory exists"],"family":"terraform","finding":"candidate_dead_iac","id":"dead_iac:terraform:mod-%d","reachability":"unused","repo_id":"repository:r1"}`, i, i)
	}
	fmt.Fprintf(&sb, `],"findings_count":%d,"limit":100,"limitations":["bounded to the requested repository scope","dynamic templates and variable-selected references are reported as ambiguous","exact dead-IaC requires reducer-materialized usage rows"],"next_offset":null,"offset":0,"repo_ids":["repository:r1"],"total_findings_count":%d,"truncated":false,"truth_basis":"content_scope"},"error":null}`, deadDerivedWorstCaseFiles, deadDerivedWorstCaseFiles)
	return sb.String()
}

// BenchmarkIaCDeadDerivedRead measures the dead leg serving the
// derived-branch worst case through the full production chain: one
// shared-key POST whose stub-core answer is the 10,000-finding page.
// The script gates it against the same-shape relay page, so the 1.50
// bound stays a real pathology check on the largest page in play.
func BenchmarkIaCDeadDerivedRead(b *testing.B) {
	_, srv := iacBenchServer(b, iacDeadDerivedWorstCaseBody())
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("dead-derived-10k", func(b *testing.B) {
		b.ReportAllocs()
		req := authedRequest(http.MethodPost, iacRoutePath(iacDeadRoute))
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req.Clone(req.Context()))
		}
	})
}

// BenchmarkIaCResourcesRead measures the resources candidate-page leg
// through the full production chain: one shared-key GET with the
// paging query the route forwards. The script gates it against the
// same post-relay before-shape as the POST legs.
func BenchmarkIaCResourcesRead(b *testing.B) {
	_, srv := iacBenchServer(b, `{"data":{"total":0},"error":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("resources", func(b *testing.B) {
		b.ReportAllocs()
		req := authedRequest(http.MethodGet, "/api/v0/iac/resources?limit=2")
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req.Clone(req.Context()))
		}
	})
}

// BenchmarkIaCRelayRead measures the relay legs through the full
// production chain: POST /api/v0/code/bundles rides the generic relay
// (a route no wave table covers, drift-style), so it is the
// before-shape for every cleared leg above — no IaC read rides the
// relay now that the dead leg terminates natively. Both legs do
// exactly one server-side fetch, so the 1.50 bound stays a real
// pathology check either way. The 10k leg answers the same worst-case
// derived page as the dead derived leg, so the large-page comparison
// is same-shape.
func BenchmarkIaCRelayRead(b *testing.B) {
	b.Run("post-relay", func(b *testing.B) {
		_, srv := iacBenchServer(b, `{"data":{"total":0},"error":null}`)
		handler := srv.Handler(DefaultV0Sunset)

		b.ReportAllocs()
		req := authedRequest(http.MethodPost, "/api/v0/code/bundles")
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req.Clone(req.Context()))
		}
	})
	b.Run("post-relay-10k", func(b *testing.B) {
		_, srv := iacBenchServer(b, iacDeadDerivedWorstCaseBody())
		handler := srv.Handler(DefaultV0Sunset)

		b.ReportAllocs()
		req := authedRequest(http.MethodPost, "/api/v0/code/bundles")
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req.Clone(req.Context()))
		}
	})
}
