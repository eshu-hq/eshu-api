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

// Drift relay-leg termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#117). TDD record: the frozen wave fails
// RED before the implementation lands (driftWave undefined), and the
// ahead-of-relay assertions fail next (the 2 drift routes still cross
// the generic relay); both are green with the wave gate registered
// ahead of the generic proxy. Until the lane lands, every drift call
// crosses the generic byte-relay, so the ahead-of-relay assertions
// fail first.

// TestDriftWaveFrozen pins the exact surface the satellite
// terminates: the findings POST plus the packet GET behind
// CloudRuntimeDriftReadAdmission v0 (2 re-declared methods: the
// bounded active-finding page plus the truncation total for
// /cloud/runtime-drift/findings; the page alone for
// /investigations/drift/packet, which carries no total by
// construction). A wave change moves the ownership note, the
// fragment reference, and this mirror together — the same
// frozen-set pattern as TestCodeSearchWaveFrozen.
func TestDriftWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/investigations/drift/packet evidence gated go/internal/query/openapi/paths/evidence/investigations.go",
		"POST /api/v0/cloud/runtime-drift/findings iac gated go/internal/query/openapi/paths/cloud/runtime_drift.go",
	}
	var got []string
	for _, entry := range driftWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("driftWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("driftWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// driftFindingsPath is the wave findings POST; driftPacketPath is the
// wave packet GET.
const (
	driftFindingsPath = "/api/v0/cloud/runtime-drift/findings"
	driftPacketPath   = "/api/v0/investigations/drift/packet"
)

// driftFindingsBodies are the well-formed request bodies the behavior
// tests drive through the findings leg: the canonical scope shape,
// the provider-alias shape, and the filtered paged shape. The
// satellite forwards every shape opaquely; core validates them.
var driftFindingsBodies = []string{
	`{"scope_id":"cloud-scope:gcp:project-synthetic","limit":10}`,
	`{"project_id":"cloud-scope:gcp:project-synthetic","provider":"gcp","limit":10}`,
	`{"scope_id":"cloud-scope:aws:123456789012","provider":"aws","limit":500,"offset":100}`,
}

// driftPacketTargets are the well-formed query targets the behavior
// tests drive through the packet leg: the canonical scope shape, the
// provider-alias shape, and the filtered shape.
var driftPacketTargets = []string{
	"/api/v0/investigations/drift/packet?scope_id=cloud-scope:gcp:project-synthetic",
	"/api/v0/investigations/drift/packet?project_id=cloud-scope:gcp:project-synthetic&provider=gcp",
	"/api/v0/investigations/drift/packet?scope_id=cloud-scope:aws:123456789012&provider=aws&cloud_resource_uid=uid-1",
}

// driftAuthedPost builds a shared-key drift findings POST with one of
// the parity bodies, the negotiated shape, and a pinned correlation
// ID.
func driftAuthedPost(body, accept string) *http.Request {
	req := authedRequest(http.MethodPost, driftFindingsPath)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "drift-parity-1")
	return req
}

// driftAuthedGet builds a shared-key drift packet GET with one of the
// parity query targets, the negotiated shape, and a pinned
// correlation ID.
func driftAuthedGet(target, accept string) *http.Request {
	req := authedRequest(http.MethodGet, target)
	req.Header.Set("Accept", accept)
	req.Header.Set("X-Correlation-ID", "drift-parity-1")
	return req
}

// TestDriftWaveAheadOfRelay pins that the wave routes are explicit
// registrations ahead of the generic proxy: a shared-key call must
// file its dispatch on the native lane (viaNative), not the relay
// lane — the findings leg on the iac family, the packet leg on the
// evidence family. Until the lane lands, the calls cross the generic
// relay and this test is RED.
func TestDriftWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, body := range driftFindingsBodies {
		t.Run("POST findings", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"drift_findings":[],"findings_count":0}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, driftAuthedPost(body, "application/json"))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			if core.lastBody != body {
				t.Errorf("upstream body = %q, want the drift request %q forwarded intact", core.lastBody, body)
			}
			if core.lastPath != driftFindingsPath {
				t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, driftFindingsPath)
			}
			snap := tel.snapshot()
			lanes, ok := snap.dispatches["iac"]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", "iac")
			}
			if lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", "iac", lanes)
			}
			if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1", "iac", got)
			}
		})
	}
	for _, target := range driftPacketTargets {
		t.Run("GET packet", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"packet":{},"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, driftAuthedGet(target, "application/json"))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			if core.lastPath != driftPacketPath {
				t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, driftPacketPath)
			}
			snap := tel.snapshot()
			lanes, ok := snap.dispatches["evidence"]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", "evidence")
			}
			if lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", "evidence", lanes)
			}
			if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1", "evidence", got)
			}
		})
	}
}

// TestDriftSlashSpellingsTerminateNatively pins the never-split rule
// for the wave: every trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestDriftSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for _, entry := range driftWave {
		t.Run(entry.Method+" "+entry.Path+" slash", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"drift_findings":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			var req *http.Request
			if entry.Method == http.MethodPost {
				req = driftAuthedPost(driftFindingsBodies[0], "application/json")
				req.URL.Path = entry.Path + "/"
			} else {
				req = driftAuthedGet(entry.Path+"/"+"?scope_id=cloud-scope:gcp:project-synthetic", "application/json")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("%s %s/: status = %d, want 200 from the stub core", entry.Method, entry.Path, rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.dispatches[entry.Family][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", entry.Family, got)
			}
		})
	}
}

// TestDriftTerminated_ForwardsCallerBytes pins that the terminated
// reader forwards the caller's method, path, query, negotiation
// headers, credential, and body to the same core route: exactly one
// upstream hit, one family authorizer allow, no dest retry, no
// credential widening — for the findings POST and the packet GET.
func TestDriftTerminated_ForwardsCallerBytes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"drift_findings":[],"findings_count":0}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	req := driftAuthedPost(driftFindingsBodies[2], EnvelopeMIME)
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
	if core.lastPath != driftFindingsPath {
		t.Errorf("upstream path = %q, want the wave path", core.lastPath)
	}
	if core.lastBody != driftFindingsBodies[2] {
		t.Errorf("upstream body = %q, want the caller body intact", core.lastBody)
	}
	if core.lastAccept != EnvelopeMIME {
		t.Errorf("upstream Accept = %q, want the negotiated shape intact", core.lastAccept)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization carries the caller credential forward, got %q", core.lastAuth)
	}
	if got := rec.Header().Get("X-Correlation-ID"); got != "drift-parity-1" {
		t.Errorf("response correlation = %q, want the caller value echoed", got)
	}

	get := driftAuthedGet(driftPacketTargets[2], "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, get)

	if rec.Code != http.StatusOK {
		t.Fatalf("packet status = %d, want 200 from the stub core", rec.Code)
	}
	if core.hits != 2 {
		t.Errorf("upstream hits = %d, want exactly 2 total (one per call, no dest retry)", core.hits)
	}
	if core.lastQuery != "scope_id=cloud-scope:aws:123456789012&provider=aws&cloud_resource_uid=uid-1" {
		t.Errorf("upstream query = %q, want the caller query intact", core.lastQuery)
	}

	snap := tel.snapshot()
	if got := snap.authorizer["iac"][decisionAllow]; got != 1 {
		t.Errorf("authorizer[iac][allow] = %d, want exactly 1 for the terminated findings call", got)
	}
	if got := snap.authorizer["evidence"][decisionAllow]; got != 1 {
		t.Errorf("authorizer[evidence][allow] = %d, want exactly 1 for the terminated packet call", got)
	}
}

// driftParityBodies are the caller bodies driven through the findings
// leg of the byte-parity matrix.
var driftParityBodies = []string{
	`{"scope_id":"cloud-scope:gcp:project-synthetic","limit":10}`,
	`{"project_id":"cloud-scope:gcp:project-synthetic","provider":"gcp","limit":10}`,
}

// driftParityTargets are the caller query targets driven through the
// packet leg of the byte-parity matrix.
var driftParityTargets = []string{
	"/api/v0/investigations/drift/packet?scope_id=cloud-scope:gcp:project-synthetic",
	"/api/v0/investigations/drift/packet?project_id=cloud-scope:gcp:project-synthetic&provider=gcp",
}

// TestDriftTerminatedMatchesRelayByteForByte pins byte parity between
// the terminated wave lane and the generic relay for both terminated
// routes: status, headers, and body cross identically in both
// negotiated shapes. The stub core answers every documented drift
// outcome (200 found and scoped-empty zero page, 200 built and
// refusal packets, 400 validation/malformed, capability and
// read-model 501s, fixed-message 500s, retryable 503 with
// Retry-After, gateway 504), so the lane proves parity on the full
// outcome set the arbiter bar names without inventing any handler
// behavior dest-side.
func TestDriftTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	postOutcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"plain 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"drift_findings":[{"fact_id":"f1"}],"findings_count":1,"total_findings_count":1}`},
		{"scoped-empty zero page 200", http.StatusOK, nil, `{"drift_findings":[],"findings_count":0,"total_findings_count":0,"truncated":false}`},
		{"validation 400 missing scope", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"scope_id, account_id, project_id, or subscription_id is required"}`},
		{"validation 400 bad provider", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"provider must be one of aws, gcp, or azure"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"cloud runtime drift readback requires reducer-materialized provider-neutral drift facts"}`},
		{"read-model 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"cloud runtime drift readback requires the reducer drift finding read model"}`},
		{"fixed-message 500 readback failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"cloud runtime drift readback failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
	getOutcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"built packet 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"packet":{"family":"drift"},"truth":null}`},
		{"refusal packet 200", http.StatusOK, nil, `{"packet":{"family":"drift","refusal":"scope_not_found"},"truth":null}`},
		{"validation 400 missing scope", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"scope_id, account_id, project_id, or subscription_id is required"}`},
		{"validation 400 bad provider", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"provider must be one of aws, gcp, or azure"}`},
		{"capability 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"drift packets require reducer-materialized provider-neutral drift facts"}`},
		{"read-model 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"drift packets require the reducer drift finding read model"}`},
		{"fixed-message 500 packet read failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"drift packet readback failed"}`},
		{"fixed-message 500 packet build failed", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"drift packet build failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"gateway 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"core read exceeded its deadline"}`},
	}
	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		for _, body := range driftParityBodies {
			for _, outcome := range postOutcomes {
				name := "POST findings " + outcome.name + " " + accept
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
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, driftAuthedPost(body, accept))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, driftAuthedPost(body, accept))

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
		for _, target := range driftParityTargets {
			for _, outcome := range getOutcomes {
				name := "GET packet " + outcome.name + " " + accept
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
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, driftAuthedGet(target, accept))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, driftAuthedGet(target, accept))

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
