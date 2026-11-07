// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// D9 SAML-login tier-C clone resilience proof (Refs eshu-hq/eshu#9669,
// part of #6707). The clone holds the same failure contract as the D8
// and GitHub lanes: fail closed with no core configured, exactly one
// upstream attempt on every failure shape, and no dest retry on
// cancellation or an unstable core. Transport 500s/503s are retryable
// only end-to-end by the client, never by dest. Start and ACS are
// census mutations (login start creates server-side state; an ACS
// consume spends the request core-side), so the single-attempt rule is
// the whole exactly-once story dest-side — including for the ACS POST
// carrying a large form body.

// d9SAMLSpellings returns every native spelling: each exact route —
// and no slash twin (a twin would be a subtree swallowing past the auth
// bypass, so those spellings stay on the generic relay by design,
// pinned in the public suite).
func d9SAMLSpellings() []struct {
	method      string
	path        string
	body        string
	contentType string
} {
	var out []struct {
		method      string
		path        string
		body        string
		contentType string
	}
	for _, route := range d9SAMLRoutes {
		method, path, _ := strings.Cut(route, " ")
		var body, ctype string
		if method == http.MethodPost {
			body, ctype = "RelayState=opaque-relay& SAMLResponse=opaque-response", "application/x-www-form-urlencoded"
		}
		out = append(out, struct {
			method      string
			path        string
			body        string
			contentType string
		}{method, path, body, ctype})
	}
	return out
}

// TestD9SAMLNative_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: with no core origin configured all three native spellings
// answer the permanent 503 backend_unavailable instead of panicking,
// in both negotiated shapes.
func TestD9SAMLNative_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range d9SAMLSpellings() {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			var req *http.Request
			if spelling.body != "" {
				req = httptest.NewRequest(spelling.method, spelling.path, strings.NewReader(spelling.body))
				req.Header.Set("Content-Type", spelling.contentType)
			} else {
				req = httptest.NewRequest(spelling.method, spelling.path, nil)
			}
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("%s %q Accept %q: panicked with %v, want 503", spelling.method, spelling.path, accept, recovered)
					}
				}()
				handler.ServeHTTP(rec, req)
			}()

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s %q Accept %q: status = %d, want 503", spelling.method, spelling.path, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%s %q Accept %q: Retry-After = %q, want absent (permanent 503)", spelling.method, spelling.path, accept, got)
			}
			respBody, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(respBody), "backend_unavailable") {
				t.Errorf("%s %q Accept %q: body = %q, want backend_unavailable", spelling.method, spelling.path, accept, respBody)
			}
		}
	}
}

// d9SAMLAttemptCountingCases is the exactly-once matrix for the ACS
// POST: every unstable shape costs exactly one upstream attempt.
func d9SAMLAttemptCountingCases() []struct {
	name   string
	status int
	header http.Header
	body   string
} {
	return []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"core 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"saml login failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
		{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the session store is not configured"}`},
		{"acs refused 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"RelayState and SAMLResponse are required"}`},
	}
}

// TestD9SAMLNative_ACSPostAttemptsOnce pins POST attempt counting: on
// a 500, a retryable 503, a permanent 503, and core's own form-400,
// the ACS POST costs exactly one upstream attempt and the core bytes
// relay unmodified (Retry-After included). Dest never retries a
// census mutation: a retried ACS would spend the request twice
// core-side.
func TestD9SAMLNative_ACSPostAttemptsOnce(t *testing.T) {
	t.Parallel()

	for _, tc := range d9SAMLAttemptCountingCases() {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			var attempts int32
			srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
			handler := srv.Handler(DefaultV0Sunset)

			form := "RelayState=opaque-relay& SAMLResponse=opaque-response"
			req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := atomic.LoadInt32(&attempts); got != 1 {
				t.Errorf("%s Accept %q: dispatch attempts = %d, want exactly 1 (no retry)", tc.name, accept, got)
			}
			if rec.Code != tc.status {
				t.Errorf("%s Accept %q: status = %d, want relayed %d", tc.name, accept, rec.Code, tc.status)
			}
			if tc.header.Get("Retry-After") != "" {
				if got := rec.Header().Get("Retry-After"); got != tc.header.Get("Retry-After") {
					t.Errorf("%s Accept %q: Retry-After = %q, want the core's value relayed", tc.name, accept, got)
				}
			}
			respBody, _ := io.ReadAll(rec.Result().Body)
			if string(respBody) != tc.body {
				t.Errorf("%s Accept %q: body = %q, want the core bytes %q", tc.name, accept, respBody, tc.body)
			}
		}
	}
}

// TestD9SAMLNative_DeadCoreIs503WithoutRetry pins the dead-core shape
// on the native ACS route: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count
// proving exactly one dispatch attempt (no retry on a census
// mutation).
func TestD9SAMLNative_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		form := "RelayState=opaque-relay& SAMLResponse=opaque-response"
		req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := atomic.LoadInt32(&attempts); got != 1 {
			t.Errorf("Accept %q: dispatch attempts = %d, want exactly 1 (no retry)", accept, got)
		}
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Accept %q: status = %d, want 503", accept, rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Errorf("Accept %q: Retry-After = %q, want absent (permanent 503)", accept, got)
		}
		respBody, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(respBody), "backend_unavailable") {
			t.Errorf("Accept %q: body = %q, want backend_unavailable", accept, respBody)
		}
	}
}

// TestD9SAMLNative_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native routes: a canceled caller
// gets the permanent 503 backend_unavailable with no retry, pre-cancel
// and mid-flight, with the attempt count proving a single dispatch
// attempt in both cases. The mid-flight row runs the ACS POST with
// its form body, proving the streaming body costs no second attempt.
func TestD9SAMLNative_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cancel string // "before" serves a pre-canceled context; "during" cancels mid-flight
	}{
		{"pre-cancel", "before"},
		{"mid-flight", "during"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(500 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			}))
			defer slow.Close()

			srv := NewServer(slow.URL, skeletonTestKey)
			var attempts int32
			srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
			handler := srv.Handler(DefaultV0Sunset)

			ctx, cancel := context.WithCancel(context.Background())
			if tc.cancel == "before" {
				cancel()
			} else {
				defer cancel()
				go func() {
					time.Sleep(100 * time.Millisecond)
					cancel()
				}()
			}
			form := "RelayState=opaque-relay& SAMLResponse=opaque-response"
			req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(form)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := atomic.LoadInt32(&attempts); got != 1 {
				t.Errorf("dispatch attempts = %d, want exactly 1 (no retry)", got)
			}
			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", rec.Code)
			}
			respBody, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(respBody), "backend_unavailable") {
				t.Errorf("body = %q, want backend_unavailable", respBody)
			}
		})
	}
}

// TestD9SAMLNative_ParallelProbe pins that concurrent callers each get
// one write-through: 16 parallel login starts answer 302 with the
// relayed core Location and land 16 upstream hits (one forward per
// caller, no dest coalescing and no dest retry). Dest holds no state
// store or replay cache, so it serializes nothing — which is exactly
// the no-worse-than-core proof: the same burst against the core
// directly converges the same way.
func TestD9SAMLNative_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Location", "https://idp.example.com/sso?SAMLRequest=opaque")
		w.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	const callers = 16
	var wg sync.WaitGroup
	failures := make([]string, callers)
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusFound {
				failures[i] = fmt.Sprintf("caller %d: status = %d, want 302", i, rec.Code)
				return
			}
			if got := rec.Header().Get("Location"); got != "https://idp.example.com/sso?SAMLRequest=opaque" {
				failures[i] = fmt.Sprintf("caller %d: Location = %q, want the relayed core value", i, got)
			}
		}(i)
	}
	wg.Wait()

	for _, failure := range failures {
		if failure != "" {
			t.Error(failure)
		}
	}
	if got := hits.Load(); got != callers {
		t.Errorf("upstream hits = %d, want %d (one write-through per caller, no retry)", got, callers)
	}
}

// d9SAMLACSOnceStub mimics the core's current-state behavior behind
// the ACS route: every delivery executes (dest dedups nothing — a
// retried ACS spends the request core-side), and the stub answers every
// delivery the same way so the test proves dest adds nothing — no dedup
// cache, no retry, no coalescing.
type d9SAMLACSOnceStub struct {
	hits int
}

func (s *d9SAMLACSOnceStub) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.hits++
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"csrf_token":"opaque-csrf"}`)
	})
}

// TestD9SAMLNative_DuplicateIsNoWorseThanCore pins the tier-C
// proof-box duplicate rule: a retried ACS lands exactly as it would
// against the core directly, because dest adds nothing — no dedup
// cache, no retry, no coalescing. Two identical deliveries forward
// twice (2 upstream hits) and both land. The ledger deficit stays open
// in the contract: this test proves dest is no worse than direct core,
// not that a retried ACS is safe (core spends the request on consume).
func TestD9SAMLNative_DuplicateIsNoWorseThanCore(t *testing.T) {
	t.Parallel()

	stub := &d9SAMLACSOnceStub{}
	upstream := httptest.NewServer(stub.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	form := "RelayState=opaque-relay& SAMLResponse=opaque-response"
	var codes []int
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	if stub.hits != 2 {
		t.Errorf("upstream hits = %d, want 2 (dest forwards both, dedups neither)", stub.hits)
	}
	if codes[0] != codes[1] {
		t.Errorf("statuses = %v, want both deliveries to land the same", codes)
	}
}

// TestD9SAMLNative_DisabledProviderRelaysCoreAnswer pins the
// conditional mount: core mounts the SAML routes only when the
// provider runtime is configured (`if a.SAML != nil` in
// `APIRouter.Mount`, nil when the store is unset), so a disabled
// provider leaves the paths unmounted and the mux answers 404. Dest
// relays core's real unmounted answer — the stub below 404s every path
// exactly like an unmounted core mux — with exactly one upstream hit
// per carved request, keyless, synthesizing nothing: no dest 401 (the
// carve-out serves keyless callers), no invented body, no retry.
func TestD9SAMLNative_DisabledProviderRelaysCoreAnswer(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	unmounted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "404 page not found\n")
	}))
	defer unmounted.Close()

	srv := NewServer(unmounted.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	requests := 0
	for _, tc := range d9SAMLPublicCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			target := tc.path
			if tc.query != "" {
				target += "?" + tc.query
			}
			var req *http.Request
			if tc.body != "" {
				req = httptest.NewRequest(tc.method, target, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", tc.contentType)
			} else {
				req = httptest.NewRequest(tc.method, target, nil)
			}
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			requests++

			if rec.Code != http.StatusNotFound {
				t.Errorf("%s Accept %q: status = %d, want core's unmounted 404 relayed", tc.name, accept, rec.Code)
			}
			respBody, _ := io.ReadAll(rec.Result().Body)
			if string(respBody) != "404 page not found\n" {
				t.Errorf("%s Accept %q: body = %q, want core's unmounted answer (dest synthesizes nothing)", tc.name, accept, respBody)
			}
		}
	}
	if got := hits.Load(); got != int32(requests) {
		t.Errorf("upstream hits = %d, want %d (one forward per carved request, no retry)",
			got, requests)
	}
}
