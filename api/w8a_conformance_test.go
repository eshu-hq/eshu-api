// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// W8a satellite termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#100, D13 tracker eshu-hq/eshu#10491).
// TDD record: the frozen wave below failed RED before the
// implementation landed (w8aWave undefined), and the ahead-of-relay
// assertions failed next (the 16 routes still crossed the generic
// relay); both are green with the wave gate registered ahead of the
// generic proxy. Until the lane lands, every W8a call crosses the
// generic byte-relay (or the key gate), so the ahead-of-relay
// assertions fail first. That red is the recorded expectation, not a
// defect.

// TestW8aWaveFrozen pins the exact surface the satellite terminates:
// the 16 W8a routes behind the seven deliberate pins
// (SecretsIAMReadAdmission v0, StatusGovernanceReadAdmission v0,
// StatusHostedReadinessReadAdmission v0, IaCReadAdmission v0,
// MetricsReadAdmission v0, SemanticSearchReadAdmission v0,
// TerraformDriftReadAdmission v0). The Compare POST
// (POST /api/v0/compare/environments) is NOT in this wave: it
// terminated in its gap-04 lane and W8a only re-verifies it (see
// api/compare_conformance_test.go). A wave change moves the
// ownership note, the core-side fragment reference, and this mirror
// together — the same frozen-set pattern as TestCompareWaveFrozen.
func TestW8aWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/metrics/timeseries metrics gated paths/status/metrics.go",
		"GET /api/v0/replatforming/selectors iac gated paths/iac/replatforming_selectors.go",
		"GET /api/v0/secrets-iam/identity-trust-chains secrets_iam gated paths/infrastructure/secrets_iam.go",
		"GET /api/v0/secrets-iam/posture-gaps secrets_iam gated paths/infrastructure/secrets_iam.go",
		"GET /api/v0/secrets-iam/posture-summary secrets_iam gated paths/infrastructure/secrets_iam.go",
		"GET /api/v0/secrets-iam/privilege-posture-observations secrets_iam gated paths/infrastructure/secrets_iam.go",
		"GET /api/v0/secrets-iam/secret-access-paths secrets_iam gated paths/infrastructure/secrets_iam.go",
		"GET /api/v0/status/answer-narration status gated paths/status/answer_narration.go",
		"GET /api/v0/status/governance status gated paths/status/governance.go",
		"GET /api/v0/status/hosted-readiness status gated paths/status/hosted_readiness.go",
		"POST /api/v0/aws/runtime-drift/findings iac gated paths/cloud/aws_runtime_drift.go",
		"POST /api/v0/replatforming/ownership-packets iac gated paths/iac/replatforming_ownership.go",
		"POST /api/v0/replatforming/plans iac gated paths/iac/replatforming.go",
		"POST /api/v0/replatforming/rollups iac gated paths/iac/replatforming_rollups.go",
		"POST /api/v0/search/semantic semantic_search gated paths/search/semantic.go",
		"POST /api/v0/terraform/config-state-drift/findings terraform gated paths/iac/terraform_config_state_drift.go",
	}
	var got []string
	for _, entry := range w8aWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("w8aWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("w8aWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// w8aGetRoutes are the wave GETs the behavior tests drive through
// the lane. Query strings ride the parity matrix (see
// w8a_parity_get_test.go); the conformance legs use bare paths.
var w8aGetRoutes = []string{
	"/api/v0/secrets-iam/identity-trust-chains",
	"/api/v0/secrets-iam/privilege-posture-observations",
	"/api/v0/secrets-iam/secret-access-paths",
	"/api/v0/secrets-iam/posture-gaps",
	"/api/v0/secrets-iam/posture-summary",
	"/api/v0/status/governance",
	"/api/v0/status/answer-narration",
	"/api/v0/status/hosted-readiness",
	"/api/v0/replatforming/selectors",
	"/api/v0/metrics/timeseries",
}

// w8aPostBodies are the wave POSTs with a well-formed body each: the
// three required keys the served fragments name, so the parity legs
// prove bodies cross intact.
var w8aPostBodies = map[string]string{
	"/api/v0/aws/runtime-drift/findings":            `{"scope_id":"scope_1","limit":50}`,
	"/api/v0/replatforming/rollups":                 `{"scope_id":"scope_1","limit":50}`,
	"/api/v0/replatforming/plans":                   `{"scope_id":"scope_1","limit":50}`,
	"/api/v0/replatforming/ownership-packets":       `{"scope_id":"scope_1","limit":50}`,
	"/api/v0/search/semantic":                       `{"query":"drift findings","limit":10}`,
	"/api/v0/terraform/config-state-drift/findings": `{"scope_id":"scope_1","limit":50}`,
}

// w8aAuthedPost builds a shared-key POST with the wave body, the
// negotiated shape, and a pinned correlation ID.
func w8aAuthedPost(path, accept string) *http.Request {
	body := w8aPostBodies[path]
	req := authedRequest(http.MethodPost, path)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "w8a-parity-1")
	return req
}

// TestW8aWaveAheadOfRelay pins that the wave routes are explicit
// registrations ahead of the generic proxy: a shared-key call must
// file its dispatch on the native lane (viaNative), not the relay
// lane. Until the lane lands, the calls cross the generic relay and
// this test is RED.
func TestW8aWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, route := range w8aGetRoutes {
		t.Run("GET "+route, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := authedRequest(http.MethodGet, route)
			req.Header.Set("X-Correlation-ID", "w8a-parity-1")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(route)
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
	for path, body := range w8aPostBodies {
		t.Run("POST "+path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, w8aAuthedPost(path, "application/json"))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			if core.lastBody != body {
				t.Errorf("upstream body = %q, want the wave request %q forwarded intact", core.lastBody, body)
			}
			snap := tel.snapshot()
			family := classifyFamily(path)
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

// TestW8aSlashSpellingsTerminateNatively pins the never-split rule
// for the wave: every trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestW8aSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for _, entry := range w8aWave {
		t.Run(entry.Method+" "+entry.Path+" slash", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			var req *http.Request
			if entry.Method == http.MethodPost {
				req = w8aAuthedPost(entry.Path+"/", "application/json")
			} else {
				req = authedRequest(http.MethodGet, entry.Path+"/")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("%s %s/: status = %d, want 200 from the stub core", entry.Method, entry.Path, rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(entry.Path)
			if got := snap.dispatches[family][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", family, got)
			}
		})
	}
}

// TestW8aTerminated_ForwardsCallerBytes pins that the terminated
// reader forwards the caller's method, path, query, negotiation
// headers, credential, and body to the same core route: exactly one
// upstream hit, no dest retry, no credential widening.
func TestW8aTerminated_ForwardsCallerBytes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := w8aAuthedPost("/api/v0/search/semantic", EnvelopeMIME)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (no dest retry)", core.hits)
	}
	if core.lastMethod != http.MethodPost {
		t.Errorf("upstream method = %q, want POST", core.lastMethod)
	}
	if core.lastPath != "/api/v0/search/semantic" {
		t.Errorf("upstream path = %q, want the wave path", core.lastPath)
	}
	if core.lastBody != w8aPostBodies["/api/v0/search/semantic"] {
		t.Errorf("upstream body = %q, want the caller body intact", core.lastBody)
	}
	if core.lastAccept != EnvelopeMIME {
		t.Errorf("upstream Accept = %q, want the negotiated shape intact", core.lastAccept)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization carries the caller credential forward, got %q", core.lastAuth)
	}
	if got := rec.Header().Get("X-Correlation-ID"); got != "w8a-parity-1" {
		t.Errorf("response correlation = %q, want the caller value echoed", got)
	}

	get := authedRequest(http.MethodGet, "/api/v0/metrics/timeseries?metric=cpu&range=1h")
	get.Header.Set("Accept", "application/json")
	get.Header.Set("X-Correlation-ID", "w8a-parity-2")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, get)

	if core.hits != 2 {
		t.Errorf("upstream hits = %d, want exactly 2 total (one per call, no dest retry)", core.hits)
	}
	if core.lastQuery != "metric=cpu&range=1h" {
		t.Errorf("upstream query = %q, want the caller query intact", core.lastQuery)
	}
}

// TestW8aExcludedRoutesStayRelay pins the Decision 7 scoping line:
// the routes the seven pins do not cover are NOT wave routes and
// must not file a native dispatch. A shared-key call to each still
// answers from the stub core on the generic relay lane — the pins
// never widen silently. (The suppressions mutation stays on its
// tier-C write-through lane; Compare terminated in its gap-04 lane.
// GET /api/v0/iac/resources left this list in the W3 IaC-6 lane: it
// terminates natively on its supplemental SearchActive pin, pinned by
// TestIaCResources_AheadOfRelay. POST /api/v0/cloud/runtime-drift/findings
// left this list in the Wave-4 Drift lane: it terminates natively on the
// CloudRuntimeDrift pin, pinned by TestDriftWaveAheadOfRelay — see
// contracts/cloud-runtime-drift-termination.md.)
func TestW8aExcludedRoutesStayRelay(t *testing.T) {
	t.Parallel()

	excluded := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v0/code/call-graph/metrics", `{"scope_id":"scope_1"}`},
	}
	for _, route := range excluded {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			t.Parallel()

			for _, entry := range w8aWave {
				if entry.Method+" "+entry.Path == route.method+" "+route.path {
					t.Fatalf("excluded route %q is in w8aWave, want it out (no pin covers it)", route.method+" "+route.path)
				}
			}

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			var req *http.Request
			if route.method == http.MethodPost {
				req = authedRequest(route.method, route.path)
				req.Body = io.NopCloser(strings.NewReader(route.body))
				req.ContentLength = int64(len(route.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = authedRequest(route.method, route.path)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(route.path)
			lanes := snap.dispatches[family]
			if lanes[viaNative] != nil {
				t.Errorf("family %q dispatches = %v, want no native lane row for the excluded route", family, lanes)
			}
		})
	}
}
