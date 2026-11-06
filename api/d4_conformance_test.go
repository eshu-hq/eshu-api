// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

// D4 Native-next reads conformance (Refs eshu-hq/eshu#9669, part of
// #6707). TDD record: this suite failed RED before the implementation
// landed (frozen wave asserted first) and is green with it — the frozen
// wave below mirrors go/cmd/api/CUTOVER_OWNERSHIP.md (CICD, Documentation,
// Infra, PackageRegistry, Repositories, Evidence,
// InvestigationWorkflows, Playbooks; Compare deferred to D6) and the
// behavior tests require every wave route to terminate natively for
// shared-key callers while relaying unresolvable identity classes
// explicitly. Until the lane lands, the wave registers nothing and every
// wave call crosses the generic byte-relay (or the key gate), so the
// ahead-of-relay assertions fail first. That red is the recorded
// expectation, not a defect.

// TestD4WaveFrozen pins the exact Native-next surface the satellite
// must terminate. A wave change (route added, removed, renamed, or
// rehomed) fails here until the ownership map, the spec fragments, and
// this mirror move together — the same frozen-set pattern as
// TestCoreReadContractsVersioned. The want table is the verbatim
// ownership-map rows (method, path, family) plus the satellite serve
// kind and the Decision 5(c) fragment that carries the route's spec
// entry.
func TestD4WaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/catalog repositories gated paths/repository/routes.go",
		"GET /api/v0/ci-cd/run-correlations cicd gated paths/cicd/routes.go",
		"GET /api/v0/ci-cd/run-correlations/count cicd gated paths/cicd/run_correlation_aggregate.go",
		"GET /api/v0/ci-cd/run-correlations/inventory cicd gated paths/cicd/run_correlation_aggregate.go",
		"GET /api/v0/cloud/resources infra gated paths/cloud/routes.go",
		"GET /api/v0/documentation/evidence-packets/{packet_id}/freshness documentation gated paths/evidence/documentation.go",
		"GET /api/v0/documentation/facts documentation gated paths/evidence/documentation.go",
		"GET /api/v0/documentation/findings documentation gated paths/evidence/documentation.go",
		"GET /api/v0/documentation/findings/count documentation gated paths/evidence/documentation_finding_aggregate.go",
		"GET /api/v0/documentation/findings/inventory documentation gated paths/evidence/documentation_finding_aggregate.go",
		"GET /api/v0/documentation/findings/{finding_id}/evidence-packet documentation gated paths/evidence/documentation.go",
		"GET /api/v0/ecosystem/overview infra gated paths/infrastructure/routes.go",
		"GET /api/v0/evidence/admission-decisions evidence gated paths/evidence/routes.go",
		"GET /api/v0/evidence/bundle evidence gated paths/evidence/bundle.go",
		"GET /api/v0/evidence/relationships/{resolved_id} evidence gated paths/evidence/routes.go",
		"GET /api/v0/infra/resources/count infra gated paths/infrastructure/resource_aggregate.go",
		"GET /api/v0/infra/resources/inventory infra gated paths/infrastructure/resource_aggregate.go",
		"GET /api/v0/investigation-workflows investigation_workflows gated paths/evidence/investigation_workflows.go",
		"GET /api/v0/investigations/deployable-unit/packet evidence gated paths/evidence/investigations.go",
		"GET /api/v0/package-registry/correlations package_registry gated paths/repository/package_registry.go",
		"GET /api/v0/package-registry/dependencies package_registry gated paths/repository/package_registry.go",
		"GET /api/v0/package-registry/dependency-chains package_registry gated paths/repository/package_registry.go",
		"GET /api/v0/package-registry/packages package_registry gated paths/repository/package_registry.go",
		"GET /api/v0/package-registry/packages/count package_registry gated paths/repository/package_registry_aggregate.go",
		"GET /api/v0/package-registry/packages/inventory package_registry gated paths/repository/package_registry_aggregate.go",
		"GET /api/v0/package-registry/versions package_registry gated paths/repository/package_registry.go",
		"GET /api/v0/query-playbooks playbooks gated paths/catalog/playbooks.go",
		"GET /api/v0/repositories repositories gated paths/repository/routes.go",
		"GET /api/v0/repositories/by-language repositories gated paths/repository/routes.go",
		"GET /api/v0/repositories/language-inventory repositories gated paths/repository/routes.go",
		"GET /api/v0/repositories/{repo_id}/branches repositories gated paths/repository/branches.go",
		"GET /api/v0/repositories/{repo_id}/content repositories gated paths/repository/routes.go",
		"GET /api/v0/repositories/{repo_id}/context repositories gated paths/repository/routes.go",
		"GET /api/v0/repositories/{repo_id}/coverage repositories gated paths/repository/stats_and_coverage.go",
		"GET /api/v0/repositories/{repo_id}/freshness repositories gated paths/repository/freshness.go",
		"GET /api/v0/repositories/{repo_id}/stats repositories gated paths/repository/stats_and_coverage.go",
		"GET /api/v0/repositories/{repo_id}/story repositories gated paths/repository/routes.go",
		"GET /api/v0/repositories/{repo_id}/tree repositories gated paths/repository/routes.go",
		"POST /api/v0/ecosystem/graph-summary infra gated paths/infrastructure/routes.go",
		"POST /api/v0/evidence/citations evidence gated paths/evidence/routes.go",
		"POST /api/v0/infra/relationships infra gated paths/infrastructure/routes.go",
		"POST /api/v0/infra/resources/search infra gated paths/infrastructure/routes.go",
		"POST /api/v0/investigation-workflows/resolve investigation_workflows gated paths/evidence/investigation_workflows.go",
		"POST /api/v0/query-playbooks/resolve playbooks gated paths/catalog/playbooks.go",
		"POST /api/v0/relationships/catalog infra gated paths/infrastructure/routes.go",
		"POST /api/v0/relationships/edges infra gated paths/infrastructure/routes.go",
	}
	if len(d4Wave) != len(want) {
		t.Fatalf("d4Wave holds %d routes, want frozen %d: move the ownership map, the fragments, and this mirror together",
			len(d4Wave), len(want))
	}
	got := make([]string, 0, len(d4Wave))
	for _, route := range d4Wave {
		got = append(got, route.Method+" "+route.Path+" "+route.Family+" "+route.Serve+" "+route.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("d4Wave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// d4ConcretePaths fills the wave's path wildcards so the behavior
// tests drive real mux matches. Trailing-slash spellings ride the same
// native registration (never-split), so each entry also names its slash
// variant where the mux distinguishes it.
var d4ConcretePaths = map[string]string{
	"GET /api/v0/catalog":                                              "/api/v0/catalog",
	"GET /api/v0/ci-cd/run-correlations":                               "/api/v0/ci-cd/run-correlations",
	"GET /api/v0/ci-cd/run-correlations/count":                         "/api/v0/ci-cd/run-correlations/count",
	"GET /api/v0/ci-cd/run-correlations/inventory":                     "/api/v0/ci-cd/run-correlations/inventory",
	"GET /api/v0/cloud/resources":                                      "/api/v0/cloud/resources",
	"GET /api/v0/documentation/evidence-packets/{packet_id}/freshness": "/api/v0/documentation/evidence-packets/pkt_1/freshness",
	"GET /api/v0/documentation/facts":                                  "/api/v0/documentation/facts",
	"GET /api/v0/documentation/findings":                               "/api/v0/documentation/findings",
	"GET /api/v0/documentation/findings/count":                         "/api/v0/documentation/findings/count",
	"GET /api/v0/documentation/findings/inventory":                     "/api/v0/documentation/findings/inventory",
	"GET /api/v0/documentation/findings/{finding_id}/evidence-packet":  "/api/v0/documentation/findings/find_1/evidence-packet",
	"GET /api/v0/ecosystem/overview":                                   "/api/v0/ecosystem/overview",
	"GET /api/v0/evidence/admission-decisions":                         "/api/v0/evidence/admission-decisions",
	"GET /api/v0/evidence/bundle":                                      "/api/v0/evidence/bundle",
	"GET /api/v0/evidence/relationships/{resolved_id}":                 "/api/v0/evidence/relationships/res_1",
	"GET /api/v0/infra/resources/count":                                "/api/v0/infra/resources/count",
	"GET /api/v0/infra/resources/inventory":                            "/api/v0/infra/resources/inventory",
	"GET /api/v0/investigation-workflows":                              "/api/v0/investigation-workflows",
	"GET /api/v0/investigations/deployable-unit/packet":                "/api/v0/investigations/deployable-unit/packet",
	"GET /api/v0/package-registry/correlations":                        "/api/v0/package-registry/correlations",
	"GET /api/v0/package-registry/dependencies":                        "/api/v0/package-registry/dependencies",
	"GET /api/v0/package-registry/dependency-chains":                   "/api/v0/package-registry/dependency-chains",
	"GET /api/v0/package-registry/packages":                            "/api/v0/package-registry/packages",
	"GET /api/v0/package-registry/packages/count":                      "/api/v0/package-registry/packages/count",
	"GET /api/v0/package-registry/packages/inventory":                  "/api/v0/package-registry/packages/inventory",
	"GET /api/v0/package-registry/versions":                            "/api/v0/package-registry/versions",
	"GET /api/v0/query-playbooks":                                      "/api/v0/query-playbooks",
	"GET /api/v0/repositories":                                         "/api/v0/repositories",
	"GET /api/v0/repositories/by-language":                             "/api/v0/repositories/by-language",
	"GET /api/v0/repositories/language-inventory":                      "/api/v0/repositories/language-inventory",
	"GET /api/v0/repositories/{repo_id}/branches":                      "/api/v0/repositories/repo_1/branches",
	"GET /api/v0/repositories/{repo_id}/content":                       "/api/v0/repositories/repo_1/content",
	"GET /api/v0/repositories/{repo_id}/context":                       "/api/v0/repositories/repo_1/context",
	"GET /api/v0/repositories/{repo_id}/coverage":                      "/api/v0/repositories/repo_1/coverage",
	"GET /api/v0/repositories/{repo_id}/freshness":                     "/api/v0/repositories/repo_1/freshness",
	"GET /api/v0/repositories/{repo_id}/stats":                         "/api/v0/repositories/repo_1/stats",
	"GET /api/v0/repositories/{repo_id}/story":                         "/api/v0/repositories/repo_1/story",
	"GET /api/v0/repositories/{repo_id}/tree":                          "/api/v0/repositories/repo_1/tree",
	"POST /api/v0/ecosystem/graph-summary":                             "/api/v0/ecosystem/graph-summary",
	"POST /api/v0/evidence/citations":                                  "/api/v0/evidence/citations",
	"POST /api/v0/infra/relationships":                                 "/api/v0/infra/relationships",
	"POST /api/v0/infra/resources/search":                              "/api/v0/infra/resources/search",
	"POST /api/v0/investigation-workflows/resolve":                     "/api/v0/investigation-workflows/resolve",
	"POST /api/v0/query-playbooks/resolve":                             "/api/v0/query-playbooks/resolve",
	"POST /api/v0/relationships/catalog":                               "/api/v0/relationships/catalog",
	"POST /api/v0/relationships/edges":                                 "/api/v0/relationships/edges",
}

// TestD4WaveRoutesAheadOfRelay pins that every wave route is an
// explicit registration ahead of the generic proxy: a shared-key call
// must file its dispatch on the native lane (viaNative), not the relay
// lane. Today the wave registers nothing, so every call crosses the
// generic relay and this test is RED.
func TestD4WaveRoutesAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, entry := range d4Wave {
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			key := entry.Method + " " + entry.Path
			target, ok := d4ConcretePaths[key]
			if !ok {
				t.Fatalf("no concrete path for wave route %q", key)
			}
			req := authedRequest(entry.Method, target)
			if entry.Method == http.MethodPost {
				req.Body = io.NopCloser(http.NoBody)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(target)
			if family != entry.Family {
				t.Fatalf("classifyFamily(%q) = %q, want wave family %q", target, family, entry.Family)
			}
			lanes, ok := snap.dispatches[family]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", family)
			}
			if lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", family, lanes)
			}
			if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1", family, got)
			}
		})
	}
}
