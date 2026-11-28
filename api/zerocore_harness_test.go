// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ProofKind names the leaf-template class a ProofCase proves. The
// P-READ/P-WRITE/P-FLOW vocabulary is the eshu-hq/eshu-api#449
// contract, defined here because it appears nowhere else in the repo:
// P-READ proves a read leaf, P-WRITE a mutation leaf, P-FLOW an
// ordered multi-request flow sharing one counter and recorder.
type ProofKind string

const (
	// ProofRead is a read-leaf proof: success emits >= 1 read span.
	ProofRead ProofKind = "P-READ"
	// ProofWrite is a mutation-leaf proof: success emits the declared
	// write spans (Max pins double-write), denial emits none.
	ProofWrite ProofKind = "P-WRITE"
	// ProofFlow is an ordered multi-request proof: zero core calls
	// across the WHOLE flow with spans in the declared order.
	ProofFlow ProofKind = "P-FLOW"
)

// ProofRequest is one request inside a ProofCase. Credential selects
// the auth lane: "valid" carries the skeleton test key, "missing"
// carries no credential, "wrong" carries a wrong key.
type ProofRequest struct {
	// Method is the HTTP method, e.g. GET or POST.
	Method string
	// Path is the request target.
	Path string
	// Credential is valid, missing, or wrong.
	Credential string
	// Body is the request body for methods that carry one.
	Body string
}

// ProofCase is one zero-core proof: a request sequence plus the
// statuses and store spans it must produce with zero core-origin
// calls. Every Kind needs both a success and a denial (401/403) case:
// a harness that never proved denial cannot tell a closed gate from a
// dead route.
type ProofCase struct {
	// Name identifies the case in failure output.
	Name string
	// Kind is the leaf-template class under proof.
	Kind ProofKind
	// Requests is the ordered request sequence.
	Requests []ProofRequest
	// WantStatuses holds the expected status per request, in order.
	WantStatuses []int
	// WantSpans holds the store-span expectations over the whole
	// case. Denial cases want nil: absolute span silence.
	WantSpans []StoreSpanExpectation
}

// buildProofRequest renders one ProofRequest as an *http.Request.
func buildProofRequest(t *testing.T, pr ProofRequest) *http.Request {
	t.Helper()
	var body io.Reader
	if pr.Body != "" {
		body = strings.NewReader(pr.Body)
	}
	req := httptest.NewRequest(pr.Method, pr.Path, body)
	if pr.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	switch pr.Credential {
	case "valid":
		req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	case "wrong":
		req.Header.Set("Authorization", "Bearer wrong-key")
	case "missing":
		// No credential: the auth gate must deny before any handler.
	default:
		t.Fatalf("unknown proof credential %q: want valid, missing, or wrong", pr.Credential)
	}
	return req
}

// RunProof executes cases through handler sharing one counter and
// recorder, and asserts per case: the statuses in order, a zero
// core-call delta (success AND denial paths alike), and the span
// expectations over the span delta. Kind rules: P-READ success needs
// >= 1 read span; P-WRITE success needs >= 1 write span on top of the
// declared Min/Max pins; P-FLOW additionally needs the declared
// (system, operation) pairs to appear in order; any all-401/403 case
// needs absolute span silence.
func RunProof(t *testing.T, handler http.Handler, counter *CoreCallCounter, rec *RecordingStoreSpans, cases []ProofCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(string(tc.Kind)+"/"+tc.Name, func(t *testing.T) {
			if len(tc.Requests) != len(tc.WantStatuses) {
				t.Fatalf("case %q: %d requests but %d want-statuses", tc.Name, len(tc.Requests), len(tc.WantStatuses))
			}
			callsBefore := counter.Calls()
			spansBefore := len(rec.Snapshot())
			for i, pr := range tc.Requests {
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, buildProofRequest(t, pr))
				if recorder.Code != tc.WantStatuses[i] {
					t.Fatalf("case %q request %d (%s %s): status = %d, want %d",
						tc.Name, i, pr.Method, pr.Path, recorder.Code, tc.WantStatuses[i])
				}
			}
			if delta := counter.Calls() - callsBefore; delta != 0 {
				t.Fatalf("case %q: core-origin calls = %d, want 0 (zero-core violation)", tc.Name, delta)
			}
			delta := rec.Snapshot()[spansBefore:]
			denied := true
			for _, status := range tc.WantStatuses {
				if status != http.StatusUnauthorized && status != http.StatusForbidden {
					denied = false
				}
			}
			if denied {
				assertStoreSpanSlice(t, delta, nil)
				return
			}
			assertStoreSpanSlice(t, delta, tc.WantSpans)
			var needOp string
			switch tc.Kind {
			case ProofRead:
				needOp = StoreOpRead
			case ProofWrite:
				needOp = StoreOpWrite
			case ProofFlow:
				assertSpanOrder(t, tc.Name, delta, tc.WantSpans)
				return
			default:
				t.Fatalf("case %q: unknown proof kind %q", tc.Name, tc.Kind)
			}
			for _, span := range delta {
				if span.Operation == needOp {
					return
				}
			}
			t.Fatalf("case %q (%s): no %q span in delta %v", tc.Name, tc.Kind, needOp, delta)
		})
	}
}

// assertSpanOrder fails t unless the declared (system, operation)
// pairs appear as a subsequence of the recorded delta, in order. It is
// the P-FLOW ordering leg: per-expectation Min/Max pins counts, this
// pins the sequence.
func assertSpanOrder(t *testing.T, name string, delta []StoreSpan, want []StoreSpanExpectation) {
	t.Helper()
	pos := 0
	for _, exp := range want {
		found := false
		for pos < len(delta) {
			span := delta[pos]
			pos++
			if span.System == exp.System && span.Operation == exp.Operation {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("case %q (P-FLOW): no (%s, %s) span after position %d in delta %v",
				name, exp.System, exp.Operation, pos, delta)
		}
	}
}

// zeroCoreFixture is the one fake store the #449 demo proves the leaf
// templates on: an in-memory widget map read and written by a named
// handler that emits store spans through the context recorder and
// never touches a core sink. It is deliberately boring: the proof is
// about the harness lanes (census, counter, spans), not about store
// realism, which the first compute leaf owns.
type zeroCoreFixture struct {
	mu      sync.Mutex
	widgets map[string]string
}

// newZeroCoreFixture builds a fixture seeded with one widget so reads
// prove against pre-existing state, not just writes the same case made.
func newZeroCoreFixture() *zeroCoreFixture {
	return &zeroCoreFixture{widgets: map[string]string{"seed": "seed-widget"}}
}

// ServeHTTP serves the widget resource: GET lists (read span), POST
// stores the {"name": ...} body (write span). Malformed bodies fail
// 400 without touching the store, so no span fires. The census root
// for the fixture claim is this method (zeroCoreFixture.ServeHTTP).
func (f *zeroCoreFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		RecordStoreSpan(r.Context(), StoreSpan{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Name: "widgets list"})
		f.mu.Lock()
		names := make([]string, 0, len(f.widgets))
		for name := range f.widgets {
			names = append(names, name)
		}
		f.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"widgets": names})
	case http.MethodPost:
		var decoded struct {
			Name string `json:"name"`
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil || json.Unmarshal(raw, &decoded) != nil || decoded.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error_code": "bad_request"})
			return
		}
		RecordStoreSpan(r.Context(), StoreSpan{System: StoreSystemPostgreSQL, Operation: StoreOpWrite, Name: "widgets store"})
		f.mu.Lock()
		f.widgets[decoded.Name] = decoded.Name
		f.mu.Unlock()
		writeJSON(w, http.StatusCreated, map[string]string{"name": decoded.Name})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error_code": "method_not_allowed"})
	}
}

// withStoreSpanRecorder injects rec into every request context: the
// test-only stand-in for the middleware the first store-backed leaf
// will wire in production.
func withStoreSpanRecorder(rec *RecordingStoreSpans, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(WithStoreSpanRecorder(r.Context(), rec)))
	})
}

// zeroCoreFixtureChain wires the proof chain from real production
// pieces: the telemetry middleware outside the shared-key auth gate
// outside the fixture, exactly the layering routes() uses for native
// lanes. Denial cases therefore prove the real RequireAuth gate.
func zeroCoreFixtureChain(srv *Server, rec *RecordingStoreSpans, fixture http.Handler) http.Handler {
	return withStoreSpanRecorder(rec, srv.instrumented(viaNative, RequireAuth(srv.apiKey, fixture)))
}

// TestZeroCore_PositiveControlRelayCountsOne is the harness honesty
// gate and must run before any zero below is believed: an authed GET
// on an unclaimed relay path falls through to dispatch/forwardToCore
// and MUST increment the counter to exactly 1 against a live httptest
// core. If this ever reports 0, the counter is blind and every
// coreCalls==0 assertion in the file is vacuous.
func TestZeroCore_PositiveControlRelayCountsOne(t *testing.T) {
	t.Parallel()

	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))
	defer core.Close()

	srv := NewServer(core.URL, skeletonTestKey)
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, authedRequest("GET", "/api/v0/zerocore-unclaimed-relay-probe"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("relay status = %d, want 200 from the httptest core", recorder.Code)
	}
	if got := counter.Calls(); got != 1 {
		t.Fatalf("core-origin calls = %d, want exactly 1 (positive control)", got)
	}
}

// TestZeroCore_FixtureCensusHolds pins the route-census half of the
// fixture proof: the fixture handler closure reaches no core sink. It
// is the ONE claim that walks test files (includeTests=true), because
// the handler itself is test-only by mandate; production claims must
// use AssertNoCorePath, which never looks at test helpers.
func TestZeroCore_FixtureCensusHolds(t *testing.T) {
	t.Parallel()

	claim := RouteClaim{
		Pattern:    "POST /api/v0/zerocore-widgets",
		Handler:    "zeroCoreFixture.ServeHTTP",
		Credential: "shared-key",
	}
	chain, err := findZeroCorePath(claim, true)
	if err != nil {
		t.Fatalf("fixture census error: %v", err)
	}
	if len(chain) != 0 {
		t.Fatalf("fixture reachable core path: %s", strings.Join(chain, " -> "))
	}
}

// TestZeroCore_FixtureProofProvesTemplates is the #449 acceptance
// demo: P-READ, P-WRITE, and P-FLOW each with a success and a denial
// case on the one fixture route, every case with coreCalls == 0.
func TestZeroCore_FixtureProofProvesTemplates(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	counter := InstallCoreCallCounter(srv)
	rec := &RecordingStoreSpans{}
	handler := zeroCoreFixtureChain(srv, rec, newZeroCoreFixture())

	RunProof(t, handler, counter, rec, []ProofCase{
		{
			Name:         "read success",
			Kind:         ProofRead,
			Requests:     []ProofRequest{{Method: http.MethodGet, Path: "/api/v0/zerocore-widgets", Credential: "valid"}},
			WantStatuses: []int{http.StatusOK},
			WantSpans: []StoreSpanExpectation{
				{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Min: 1, Max: 1},
			},
		},
		{
			Name:         "read denial",
			Kind:         ProofRead,
			Requests:     []ProofRequest{{Method: http.MethodGet, Path: "/api/v0/zerocore-widgets", Credential: "missing"}},
			WantStatuses: []int{http.StatusUnauthorized},
		},
		{
			Name:         "write success",
			Kind:         ProofWrite,
			Requests:     []ProofRequest{{Method: http.MethodPost, Path: "/api/v0/zerocore-widgets", Credential: "valid", Body: `{"name":"w1"}`}},
			WantStatuses: []int{http.StatusCreated},
			WantSpans: []StoreSpanExpectation{
				{System: StoreSystemPostgreSQL, Operation: StoreOpWrite, Min: 1, Max: 1},
			},
		},
		{
			Name:         "write denial",
			Kind:         ProofWrite,
			Requests:     []ProofRequest{{Method: http.MethodPost, Path: "/api/v0/zerocore-widgets", Credential: "wrong", Body: `{"name":"w-evil"}`}},
			WantStatuses: []int{http.StatusUnauthorized},
		},
		{
			Name: "flow success",
			Kind: ProofFlow,
			Requests: []ProofRequest{
				{Method: http.MethodPost, Path: "/api/v0/zerocore-widgets", Credential: "valid", Body: `{"name":"w-flow"}`},
				{Method: http.MethodGet, Path: "/api/v0/zerocore-widgets", Credential: "valid"},
			},
			WantStatuses: []int{http.StatusCreated, http.StatusOK},
			WantSpans: []StoreSpanExpectation{
				{System: StoreSystemPostgreSQL, Operation: StoreOpWrite, Min: 1, Max: 1},
				{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Min: 1, Max: 1},
			},
		},
		{
			Name: "flow denial",
			Kind: ProofFlow,
			Requests: []ProofRequest{
				{Method: http.MethodGet, Path: "/api/v0/zerocore-widgets", Credential: "missing"},
				{Method: http.MethodPost, Path: "/api/v0/zerocore-widgets", Credential: "wrong", Body: `{"name":"w-evil"}`},
			},
			WantStatuses: []int{http.StatusUnauthorized, http.StatusUnauthorized},
		},
	})
}

// TestZeroCore_RegisteredRouteDenialIsZeroCore proves the denial half
// on REAL registered routes: a keyless request to the D4 wave lane
// takes the standing 401 (see router.go), and keyless/wrong-key
// requests to the generic relay lane die in RequireAuth (see auth.go),
// all with 401 and zero core contact.
//
// A wrong shared-key-shaped bearer on a WAVE route is deliberately NOT
// asserted here: the wave gate reads it as a foreign credential class
// and relays explicitly for core to judge, so it answers the relay 503
// against a dead core, not a 401. The census unit is (route,
// credential class, handler) for exactly this reason.
func TestZeroCore_RegisteredRouteDenialIsZeroCore(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	wrongRelay := httptest.NewRequest("GET", "/api/v0/zerocore-unclaimed-denial-probe", nil)
	wrongRelay.Header.Set("Authorization", "Bearer wrong-key")

	for _, tc := range []struct {
		name string
		req  *http.Request
	}{
		{"wave keyless", httptest.NewRequest("GET", "/api/v0/repositories", nil)},
		{"relay keyless", httptest.NewRequest("GET", "/api/v0/zerocore-unclaimed-denial-probe", nil)},
		{"relay wrong key", wrongRelay},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := counter.Calls()
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, tc.req)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 from the auth gate", recorder.Code)
			}
			if delta := counter.Calls() - before; delta != 0 {
				t.Fatalf("core-origin calls = %d, want 0 on the denial path", delta)
			}
		})
	}
}

// TestZeroCore_HealthSmokeIsZeroCore is a SMOKE check, not a P-READ:
// GET /health answers 200 from compiled constants with zero core
// contact, but it touches no store and emits no span, so it proves
// local-only serving without proving a leaf template.
func TestZeroCore_HealthSmokeIsZeroCore(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200", recorder.Code)
	}
	counter.AssertZero(t, "GET /health smoke")
}
