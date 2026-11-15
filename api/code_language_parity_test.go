// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Code-language byte-parity matrix (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#98, W6c). The stub core answers every documented
// W6c outcome for the route, so the lane proves parity on the full
// outcome set the arbiter bar names without inventing any handler
// behavior dest-side: the graph-backed and content-only 200s with
// the page-clamped id sets, the entity-type-gate-before-grant order,
// the per-repo fallback and page-read fallback shapes, the clamped
// empty-grant page — plus validation/malformed 400s, the
// unsupported-language and unsupported-entity-type 400s, selector
// 400s, live 403s, the 501 capability, the graph-only-needs-backend
// 501, the fixed-message 500, the retryable 503 with Retry-After,
// and gateway 504s, in both negotiated shapes. (The family-403 deny
// branch is unreachable pre-D8 and stays out of this matrix;
// TestCodeLanguageDenyBranchShapePinned pins its shape.)

type codeLanguageParityOutcome struct {
	name   string
	status int
	header http.Header
	body   string
}

// codeLanguageRetryable503 is the shared retryable-core shape: the
// lane relays it with its Retry-After intact, never converting it to
// a local 503.
var codeLanguageRetryable503 = http.Header{"Retry-After": []string{"5"}}

// codeLanguageOutcomes pins the documented language-query answers the
// matrix drives: the graph-backed and content-only 200s, the
// empty-grant 200 (no backend read), the validation, language, and
// entity-type 400s, the selector 400, the live 403, the 501
// capability and graph-only-needs-backend shapes, the fixed-message
// 500, the retryable 503, and the gateway 504.
func codeLanguageOutcomes() []codeLanguageParityOutcome {
	return []codeLanguageParityOutcome{
		{"graph-backed 200", http.StatusOK, nil, `{"language":"go","entity_type":"function","query":"handleWire","results":[{"entity_id":"e1","name":"handleWire","labels":["Function"],"file_path":"src/wire/handler.go","repo_id":"repo-1","repo_name":"wire","language":"go"}],"source_backend":"graph"}`},
		{"content-only 200", http.StatusOK, nil, `{"language":"go","entity_type":"type_alias","query":"","results":[{"entity_id":"e2","name":"WireConfig","labels":["TypeAlias"],"file_path":"src/wire/config.go","repo_id":"repo-1","language":"go"}],"source_backend":"postgres_content_store"}`},
		{"grantless 200 no backend read", http.StatusOK, nil, `{"language":"go","entity_type":"function","query":"handleWire","results":[],"source_backend":"no_backend_read"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid JSON: unexpected end of JSON input"}`},
		{"validation 400 language required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"language is required"}`},
		{"validation 400 entity_type required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"entity_type is required"}`},
		{"validation 400 unsupported language", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unsupported language \"cobol\"; supported: c, cpp, csharp, dart, elixir, go, haskell, hcl, java, javascript, jsx, kotlin, perl, php, python, ruby, rust, scala, sql, swift, tsx, typescript"}`},
		{"validation 400 unsupported entity_type", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unsupported entity_type \"widget\"; supported: annotation, class, component, directory, enum, file, function, guard, impl_block, macro, module, module_attribute, protocol, protocol_implementation, repository, sql_column, sql_function, sql_index, sql_migration, sql_table, sql_trigger, sql_view, struct, terraform_backend, terraform_check, terraform_import, terraform_lock_provider, terraform_module, terraform_moved_block, terraform_removed_block, terragrunt_config, terragrunt_dependency, terragrunt_input, terragrunt_local, type_alias, type_annotation, typedef, union, variable"}`},
		{"selector 400 ungranted repo", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repository repo-9 is not in the caller grant"}`},
		{"grant-denied 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"caller grant denies repository repo-1"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"language query requires a supported query profile"}`},
		{"capability 501 graph-only needs backend", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"entity type \"repository\" requires a graph backend"}`},
		{"fixed-message 500 query failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"language query failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, codeLanguageRetryable503, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
}

// TestCodeLanguageTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for the
// terminated route: status, headers, and body cross identically in
// both negotiated shapes, for the route and every driven body shape
// (graph-backed and content-only lookups).
func TestCodeLanguageTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		for _, body := range codeLanguageBodies {
			for _, outcome := range codeLanguageOutcomes() {
				name := "POST " + codeLanguageQueryPath + " " + outcome.name + " " + accept
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					nativeCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
					nativeMux := http.NewServeMux()
					nativeMux.Handle("/", nativeCore.handler())
					nativeUpstream := httptest.NewServer(nativeMux)
					defer nativeUpstream.Close()

					relayCore := &stubCore{status: outcome.status, header: relayHeaders(outcome.header), body: outcome.body}
					relayMux := http.NewServeMux()
					relayMux.Handle("/", relayCore.handler())
					relayUpstream := httptest.NewServer(relayMux)
					defer relayUpstream.Close()

					nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
					nativeRec := httptest.NewRecorder()
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, codeLanguageAuthedPost(codeLanguageQueryPath, body, accept))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, codeLanguageAuthedPost(codeLanguageQueryPath, body, accept))

					nativeRes, relayRes := nativeRec.Result(), relayRec.Result()
					if nativeRes.StatusCode != relayRes.StatusCode {
						t.Fatalf("native status = %d, relay status = %d, want identical", nativeRes.StatusCode, relayRes.StatusCode)
					}
					nativeBody, _ := io.ReadAll(nativeRes.Body)
					relayBody, _ := io.ReadAll(relayRes.Body)
					if string(nativeBody) != string(relayBody) {
						t.Errorf("native body = %q, relay body = %q, want identical", nativeBody, relayBody)
					}
					for _, header := range []string{"Content-Type", "Retry-After", "Location", "Cache-Control", "X-Correlation-ID"} {
						if nativeRes.Header.Get(header) != relayRes.Header.Get(header) {
							t.Errorf("header %s: native = %q, relay = %q, want identical", header, nativeRes.Header.Get(header), relayRes.Header.Get(header))
						}
					}
				})
			}
		}
	}
}

// TestCodeLanguageDecision2NotOwed pins the Decision 2 split for this
// lane: there is no tier-C mutation on the W6c language surface, so
// no suppression-state filtering parity is owed. The wave entry is a
// read-only POST behind the W6c language pin; the wave carries no
// mutation route, and the parity corpus above carries no mutation
// outcome. If a mutation ever joins this family, this test must fail
// and the lane must add the Decision 2 proof its contract names.
func TestCodeLanguageDecision2NotOwed(t *testing.T) {
	t.Parallel()

	for _, entry := range codeLanguageWave {
		if entry.Method != "POST" {
			t.Errorf("wave entry %s %s: method = %q, want POST (read-only; a mutation would owe Decision 2)", entry.Method, entry.Path, entry.Method)
		}
		if entry.Path != codeLanguageQueryPath {
			t.Errorf("wave entry %s %s: unknown W6c path (a mutation here would owe Decision 2)", entry.Method, entry.Path)
		}
	}
	for _, outcome := range codeLanguageOutcomes() {
		switch outcome.status {
		case http.StatusOK, http.StatusBadRequest, http.StatusForbidden, http.StatusNotImplemented, http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		default:
			t.Errorf("outcome %q: status = %d, want a read-outcome status (no mutation outcome in this lane)", outcome.name, outcome.status)
		}
	}
}
