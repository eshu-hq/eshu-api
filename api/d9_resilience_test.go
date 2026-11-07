// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// D9 OIDC-login tier-C clone resilience proof (Refs eshu-hq/eshu#9669,
// part of #6707). The clone holds the same failure contract as the D8
// lane: fail closed with no core configured, exactly one upstream
// attempt on every failure shape, and no dest retry on cancellation or
// an unstable core. Transport 500s/503s are retryable only end-to-end
// by the client, never by dest. These GETs are census mutations (login
// start creates server-side state), so the single-attempt rule is the
// whole exactly-once story dest-side.

// d9Spellings returns every native spelling: each exact route — and no
// slash twin (a twin would be a subtree swallowing past the auth
// bypass, so those spellings stay on the generic relay by design,
// pinned in the public suite).
func d9Spellings() []struct {
	method string
	path   string
} {
	var out []struct {
		method string
		path   string
	}
	for _, route := range d9Routes {
		method, path, _ := strings.Cut(route, " ")
		out = append(out, struct {
			method string
			path   string
		}{method, path})
	}
	return out
}

// TestD9Native_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: with no core origin configured both native spellings answer
// the permanent 503 backend_unavailable instead of panicking, in both
// negotiated shapes.
func TestD9Native_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range d9Spellings() {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(spelling.method, spelling.path, nil)
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
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), "backend_unavailable") {
				t.Errorf("%s %q Accept %q: body = %q, want backend_unavailable", spelling.method, spelling.path, accept, body)
			}
		}
	}
}

// TestD9Native_DeadCoreIs503WithoutRetry pins the dead-core shape on
// the native route: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count
// proving exactly one dispatch attempt (no retry on a census
// mutation). The unstable-core half of the tier-C proof box: a 500 or
// a retryable 503 from the core answers the same way — one forward,
// core bytes relayed — and only the client may retry end-to-end
// (covered in the conformance matrix, which pins exactly one upstream
// hit on those rows).
func TestD9Native_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/callback?state=st-opaque-123&code=cd-opaque-456", nil)
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
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "backend_unavailable") {
			t.Errorf("Accept %q: body = %q, want backend_unavailable", accept, body)
		}
	}
}

// TestD9Native_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native route: a canceled caller gets
// the permanent 503 backend_unavailable with no retry, pre-cancel and
// mid-flight, with the attempt count proving a single dispatch attempt
// in both cases.
func TestD9Native_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
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
			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/login?provider_config_id=okta-dev", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := atomic.LoadInt32(&attempts); got != 1 {
				t.Errorf("dispatch attempts = %d, want exactly 1 (no retry)", got)
			}
			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), "backend_unavailable") {
				t.Errorf("body = %q, want backend_unavailable", body)
			}
		})
	}
}

// TestD9Native_ParallelProbe pins that concurrent callers each get one
// write-through: 16 parallel login starts answer 302 with the relayed
// core Location and land 16 upstream hits (one forward per caller, no
// dest coalescing and no dest retry). Dest holds no state store or
// replay cache, so it serializes nothing — which is exactly the
// no-worse-than-core proof: the same burst against the core directly
// converges the same way.
func TestD9Native_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Location", "https://idp.example.com/authorize?state=opaque")
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
			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/login?provider_config_id=okta-dev", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusFound {
				failures[i] = fmt.Sprintf("caller %d: status = %d, want 302", i, rec.Code)
				return
			}
			if got := rec.Header().Get("Location"); got != "https://idp.example.com/authorize?state=opaque" {
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

// d9DuplicateStub mimics the core's current-state behavior behind the
// OIDC routes: every delivery executes (dest dedups nothing — a
// retried login start creates a second server-side state core-side),
// and the stub answers each delivery the same way so the test proves
// dest adds nothing — no dedup cache, no retry, no coalescing.
type d9DuplicateStub struct {
	t    *testing.T
	hits int
}

func (s *d9DuplicateStub) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits++
		w.Header().Set("Location", "https://idp.example.com/authorize?state=opaque")
		w.WriteHeader(http.StatusFound)
		_, _ = io.WriteString(w, ``)
	})
}

// TestD9Native_DuplicateIsNoWorseThanCore pins the tier-C proof-box
// duplicate rule: a retried clone lands exactly as it would against
// the core directly, because dest adds nothing — no dedup cache, no
// retry, no coalescing. Two identical deliveries forward twice (2
// upstream hits) and both land. The ledger deficit stays open in the
// contract: this test proves dest is no worse than direct core, not
// that a retried login start is safe.
func TestD9Native_DuplicateIsNoWorseThanCore(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		route string
		query string
	}{
		{"GET /api/v0/auth/oidc/login", "provider_config_id=okta-dev&tenant_id=tenant_a&return_to=%2Fconsole"},
		{"GET /api/v0/auth/oidc/callback", "state=st-opaque-123&code=cd-opaque-456"},
	} {
		method, path, _ := strings.Cut(tc.route, " ")
		stub := &d9DuplicateStub{t: t}
		upstream := httptest.NewServer(stub.handler())
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		var codes []int
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(method, path+"?"+tc.query, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			codes = append(codes, rec.Code)
		}
		if stub.hits != 2 {
			t.Errorf("%s: upstream hits = %d, want 2 (dest forwards both, dedups neither)", tc.route, stub.hits)
		}
		if codes[0] != codes[1] {
			t.Errorf("%s: statuses = %v, want both deliveries to land the same", tc.route, codes)
		}
	}
}

// TestD9Native_DisabledProviderRelaysCoreAnswer pins the conditional
// mount: core mounts the OIDC routes only when the provider runtime is
// configured (`if a.OIDCLogin != nil` in `APIRouter.Mount`, nil when
// OIDC is unset per `TestNewOIDCLoginHandlerDisabledByDefault`), so a
// disabled provider leaves both paths unmounted and the mux answers
// 404. Dest relays core's real unmounted answer — the stub below 404s
// every path exactly like an unmounted core mux — with exactly one
// upstream hit, keyless, synthesizing nothing: no dest 401 (the
// carve-out serves keyless callers), no invented body, no retry.
func TestD9Native_DisabledProviderRelaysCoreAnswer(t *testing.T) {
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

	for _, tc := range d9PublicCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(tc.method, tc.path+"?"+tc.query, nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("%s Accept %q: status = %d, want core's unmounted 404 relayed", tc.name, accept, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != "404 page not found\n" {
				t.Errorf("%s Accept %q: body = %q, want core's unmounted answer (dest synthesizes nothing)", tc.name, accept, body)
			}
		}
	}
	if got := hits.Load(); got != int32(len(d9PublicCases)*2) {
		t.Errorf("upstream hits = %d, want %d (one forward per carved request, no retry)",
			got, len(d9PublicCases)*2)
	}
}

// TestD9Native_CredentialHygiene pins that dest never handles the
// credential query values: the full caller query (state, code, and
// the start IDs) relays byte-exact to the core while no RequestURI,
// query value, Location, or Set-Cookie value reaches dest logs,
// metrics, or spans. The secrets below are canaries: any dest-side
// parse, log, or telemetry of them fails this test. Dest reads no
// query values on this lane — forwardToCore forwards RequestURI()
// opaquely — so the only dest-side sighting would be a leak.
func TestD9Native_CredentialHygiene(t *testing.T) {
	t.Parallel()

	const secretState = "st-canary-9f2c4e"
	const secretCode = "cd-canary-71ab08"

	core := &stubCore{
		status: http.StatusSeeOther,
		header: http.Header{
			"Location":   []string{"/console"},
			"Set-Cookie": []string{"__Host-eshu_session=canary-session; Path=/; HttpOnly; Secure; SameSite=Strict"},
		},
		body: ``,
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	var logs bytes.Buffer
	tel := NewTelemetry(slog.New(slog.NewJSONHandler(&logs, nil)))
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	target := "/api/v0/auth/oidc/callback?state=" + secretState + "&code=" + secretCode
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want exactly 1", core.hits)
	}
	if core.lastQuery != "state="+secretState+"&code="+secretCode {
		t.Errorf("upstream query = %q, want the opaque caller query relayed byte-exact", core.lastQuery)
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want relayed 303", rec.Code)
	}

	logged := logs.String()
	for _, leaked := range []string{secretState, secretCode, "canary-session", "RequestURI", "request_uri", "Location", "Set-Cookie", "set-cookie"} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: credential material must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	metricsBody, _ := io.ReadAll(metricsRec.Result().Body)
	for _, leaked := range []string{secretState, secretCode} {
		if strings.Contains(string(metricsBody), leaked) {
			t.Errorf("/metrics contains %q: credential material must never reach metrics", leaked)
		}
	}
}
