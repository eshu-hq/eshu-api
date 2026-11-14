// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TDD-red proofs for eshu-hq/eshu-api#130 (part of eshu-hq/eshu#9669).
// Each TestR130_* below asserts the FIXED behavior, so every one must
// FAIL on the current tree and turn green with the fix. The
// TestR130_*Control cases pin adjacent behavior that already holds and
// must keep holding through the fix (in particular the PR 138 relay
// details.reason contract). This file is additive: it changes no
// production bytes and no existing test.

// r130DecodeMap decodes one response body into string maps for the
// plain (non-envelope) and envelope error shapes.
func r130DecodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
	return decoded
}

// TestR130_ReadyZDeadCoreIsNotOK pins item 1: /readyz must reflect core
// reachability. With a dead (connection-refused) core origin, /api/*
// answers the local 503, so a 200 readyz keeps the Kubernetes readiness
// probe routing traffic to a relay that cannot reach core.
func TestR130_ReadyZDeadCoreIsNotOK(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	rec := httptest.NewRecorder()
	srv.Handler(DefaultV0Sunset).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /readyz with dead core = %d, want 503 (probe must reflect core reachability)", rec.Code)
	}
}

// TestR130_ReadyZUnconfiguredCoreIsNotOK pins the fail-closed half of
// item 1: with no core origin at all, /api/* answers the local 503, so
// readyz must not answer 200 either.
func TestR130_ReadyZUnconfiguredCoreIsNotOK(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	rec := httptest.NewRecorder()
	srv.Handler(DefaultV0Sunset).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /readyz with no core = %d, want 503 (probe must reflect core reachability)", rec.Code)
	}
}

// TestR130_ReadyZLiveCoreIsOKControl pins the healthy leg the fix must
// preserve: with a reachable core, readyz keeps answering 200.
func TestR130_ReadyZLiveCoreIsOKControl(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	rec := httptest.NewRecorder()
	srv.Handler(DefaultV0Sunset).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET /readyz with live core = %d, want 200", rec.Code)
	}
}

// TestR130_Local401PlainCarriesDetails pins item 2 for the plain shape:
// a locally synthesized 401 must fill the details field the
// ErrorResponse schema already carries (openapi/components.json), so
// callers get a reason, not just a code.
func TestR130_Local401PlainCarriesDetails(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteUnauthorized(rec, httptest.NewRequest(http.MethodGet, "/api/v0/repositories", nil))

	decoded := r130DecodeMap(t, rec)
	details, ok := decoded["details"].(map[string]any)
	if !ok || len(details) == 0 {
		t.Errorf("local 401 body = %v, want a non-empty details reason beside the code", decoded)
	}
}

// TestR130_Local401EnvelopeCarriesDetails pins item 2 for the envelope
// shape: the negotiated-envelope 401 must carry error.details too.
func TestR130_Local401EnvelopeCarriesDetails(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v0/repositories", nil)
	req.Header.Set("Accept", EnvelopeMIME)
	rec := httptest.NewRecorder()
	WriteUnauthorized(rec, req)

	decoded := r130DecodeMap(t, rec)
	errSection, ok := decoded["error"].(map[string]any)
	if !ok {
		t.Fatalf("local 401 envelope body has no error section: %v", decoded)
	}
	details, ok := errSection["details"].(map[string]any)
	if !ok || len(details) == 0 {
		t.Errorf("local 401 envelope error = %v, want a non-empty details reason beside the code", errSection)
	}
}

// TestR130_Local403CarriesDetails pins item 2 for local 403s: a
// dest-synthesized deny (here the suppression family authorizer) must
// fill details. writeError has no details path today, so this fails.
func TestR130_Local403CarriesDetails(t *testing.T) {
	t.Parallel()

	srv := NewServer("https://core:8443", skeletonTestKey)
	req := httptest.NewRequest(http.MethodPost, "/api/v0/supply-chain/impact/suppressions", nil)
	rec := httptest.NewRecorder()
	srv.suppressionWriteThroughAs(rec, req, false)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("deny status = %d, want 403", rec.Code)
	}
	decoded := r130DecodeMap(t, rec)
	details, ok := decoded["details"].(map[string]any)
	if !ok || len(details) == 0 {
		t.Errorf("local 403 body = %v, want a non-empty details reason beside the code", decoded)
	}
}

// TestR130_LocalNative503CarriesDetails pins item 2 for local native
// 503s: a dest-synthesized fail-closed 503 off the relay lane (here the
// native suppression route with no core origin) must fill details, just
// as the relay-lane 503 already does since PR 138.
func TestR130_LocalNative503CarriesDetails(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	req := httptest.NewRequest(http.MethodPost, "/api/v0/supply-chain/impact/suppressions", nil)
	rec := httptest.NewRecorder()
	srv.suppressionWriteThroughAs(rec, req, true)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil-core native status = %d, want 503", rec.Code)
	}
	decoded := r130DecodeMap(t, rec)
	details, ok := decoded["details"].(map[string]any)
	if !ok || len(details) == 0 {
		t.Errorf("local native 503 body = %v, want a non-empty details reason beside the code", decoded)
	}
}

// TestR130_Relay503KeepsReasonControl pins the PR 138 contract the item
// 2 fix must not regress: the relay-lane 503 already carries
// details.reason, and non-relay relay bytes stay untouched. Reconciling
// item 2 extends details to local errors without changing this shape.
func TestR130_Relay503KeepsReasonControl(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	req := httptest.NewRequest(http.MethodPost, "/api/v0/repositories", nil)
	rec := httptest.NewRecorder()
	srv.dispatch(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("relay status = %d, want 503", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"reason":"unconfigured"`) {
		t.Errorf("relay 503 body = %q, want details.reason unconfigured (PR 138 contract)", body)
	}
}

// TestR130_HealthHasNoDuplicateContractKey pins item 4: two health keys
// (admission_contracts and suppression_admission_contracts) carry the
// same SuppressionAdmissionContractVersion value. Exactly one spelling
// must survive the dedupe; this test is neutral about which one.
func TestR130_HealthHasNoDuplicateContractKey(t *testing.T) {
	t.Parallel()

	srv := NewServer("https://core:8443", skeletonTestKey)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler(DefaultV0Sunset).ServeHTTP(rec, req)

	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read /health body: %v", err)
	}
	var health map[string]string
	if err := json.Unmarshal(body, &health); err != nil {
		t.Fatalf("decode /health body %q: %v", body, err)
	}
	legacy, hasLegacy := health["admission_contracts"]
	explicit, hasExplicit := health["suppression_admission_contracts"]
	if hasLegacy && hasExplicit && legacy == explicit {
		t.Errorf("/health carries both admission_contracts and suppression_admission_contracts with the same value %q: dedupe to one key", legacy)
	}
}

// TestR130_ReadclientDocDoesNotClaimExclusivity pins item 5: the
// readclient package doc calls itself the only path to core truth, but
// the dispatch lane (api/dispatch.go, raw httpClient) and the native
// termination lanes reach core without it — nothing outside tests
// imports internal/readclient. The doc must stop claiming exclusivity.
func TestR130_ReadclientDocDoesNotClaimExclusivity(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../internal/readclient/doc.go")
	if err != nil {
		t.Fatalf("read readclient doc: %v", err)
	}
	if strings.Contains(string(raw), "the only path") {
		t.Errorf("internal/readclient/doc.go still claims to be the only path to core truth while dispatch bypasses it")
	}
}

// TestR130_ApiDocDoesNotRouteReadsThroughReadclient pins the api-side
// half of item 5: api/doc.go describes versioned read-contract use as
// going through internal/readclient, which no production code calls.
// The fix must reword that clause; any rewording passes.
func TestR130_ApiDocDoesNotRouteReadsThroughReadclient(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatalf("read api doc: %v", err)
	}
	if strings.Contains(string(raw), "called through internal/readclient for") {
		t.Errorf("api/doc.go still routes versioned read-contract use through internal/readclient, which production code never calls")
	}
}
