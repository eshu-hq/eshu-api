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

// ackPayload is the operator body the native route forwards: the failed
// target, the reason, and the idempotency key.
const ackPayload = `{"collector_kind":"security_alert","instance_id":"i1","scope_id":"s1","reason":"known vendor outage, tracked","idempotency_key":"k1"}`

// TestTerminalFailureNative_UnauthenticatedIs401 pins that the native
// route keeps the dest 401 shape: envelope callers get the envelope
// error, everyone else the plain error_code body, both with the bare
// Bearer challenge.
func TestTerminalFailureNative_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v0/admin/collector-failures/acknowledge", strings.NewReader(ackPayload))
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Accept %q: status = %d, want 401", accept, rec.Code)
		}
		if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Errorf("Accept %q: WWW-Authenticate = %q, want bare Bearer", accept, got)
		}
	}
}

// TestTerminalFailureNative_AuthorizerRejectsNonAllScopes pins the family
// 403 the dest authorizer adds: a non-AllScopes identity never reaches
// the core. Dest has no scoped/session resolver, so the deny branch is
// unreachable in production (the shared key maps to AllScopes in core,
// which re-judges admin scope and the scoped-token deny as backstop);
// the test pins the documented shape, not a live path.
func TestTerminalFailureNative_AuthorizerRejectsNonAllScopes(t *testing.T) {
	t.Parallel()

	if !authorizeTerminalFailureAdmin(true) {
		t.Errorf("authorizeTerminalFailureAdmin(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeTerminalFailureAdmin(false) {
		t.Errorf("authorizeTerminalFailureAdmin(false) = allow, want deny (family 403)")
	}
}

// TestTerminalFailureNative_DenyBranchShapePinned pins the deny shape for
// the day dest gains a scoped/session resolver: a non-AllScopes identity
// is denied with the documented shape and the core is never reached, in
// both negotiated shapes. The branch is unreachable on the live route
// today (the route passes true; the live 403s are core's), so this pins
// the contract, not a live path.
func TestTerminalFailureNative_DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := authedRequest(http.MethodPost, "/api/v0/admin/collector-failures/acknowledge")
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		srv.terminalFailureWriteThroughAs(rec, req, false)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Accept %q: status = %d, want 403", accept, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "all-scopes operator authorization is required") {
			t.Errorf("Accept %q: body = %q, want the family-403 message", accept, body)
		}
	}
}

// TestTerminalFailureNative_WriteThroughRelaysCoreBytes pins the native
// write-through: method, path, query, headers (including the original
// Authorization header), and body forward to the same core route, and the
// core's status, headers, and body come back untouched across the pinned
// outcomes — 200 acknowledged, 200 duplicate, the 400 validation
// rejections, the core 403, the 404 pair (unclaimed and post-claim with
// key_consumed), the 409 pair (different-target and in-progress), the 422
// refusal, and the core 503 (whose absent Retry-After stays absent,
// proving dest never invents the retry signal).
func TestTerminalFailureNative_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		body   string
		want   string // required body marker
	}{
		{"acknowledged", http.StatusOK, `{"status":"acknowledged","idempotency_key":"k1","acknowledgement":{"collector_kind":"security_alert","instance_id":"i1","scope_id":"s1","work_item_id":"w1","failure_class":"fetch_error","failure_operation":"fetch","failed_at":"2026-11-05T00:00:00Z","acknowledged_at":"2026-11-05T00:01:00Z"}}`, `"acknowledged"`},
		{"duplicate", http.StatusOK, `{"status":"duplicate","idempotency_key":"k1","acknowledgement":{"collector_kind":"security_alert","instance_id":"i1","scope_id":"s1","work_item_id":"w1","failure_class":"fetch_error","failure_operation":"fetch","failed_at":"2026-11-05T00:00:00Z","acknowledged_at":"2026-11-05T00:01:00Z"}}`, `"duplicate"`},
		{"missing reason", http.StatusBadRequest, `{"error":"Bad Request","detail":"reason is required and must say why the failure is expected"}`, "reason is required"},
		{"missing key", http.StatusBadRequest, `{"error":"Bad Request","detail":"idempotency_key is required to make acknowledgement safe under retries"}`, "idempotency_key is required"},
		{"bad target", http.StatusBadRequest, `{"error":"Bad Request","detail":"collector_kind, instance_id, and scope_id are required, and reason is at most 512 characters"}`, "collector_kind, instance_id, and scope_id are required"},
		{"core 403", http.StatusForbidden, `{"error":"Forbidden","detail":"acknowledging a collector terminal failure requires an admin (all-scopes) token"}`, "all-scopes"},
		{"404 unclaimed", http.StatusNotFound, `{"error":"Not Found","detail":"collector instance is not registered or not active for that collector_kind"}`, "not registered or not active"},
		{"404 claimed", http.StatusNotFound, `{"error":"Not Found","detail":"collector instance is not registered or not active for that collector_kind","key_consumed":true,"key_detail":"nothing was acknowledged, the idempotency key is now consumed, and a retry needs a new idempotency_key"}`, "key_consumed"},
		{"409 different target", http.StatusConflict, `{"error":"Conflict","detail":"idempotency_key was already used with a different collector_kind, instance_id, or scope_id"}`, "already used with a different"},
		{"409 in progress", http.StatusConflict, `{"error":"Conflict","detail":"an acknowledgement for this idempotency_key is already in progress"}`, "already in progress"},
		{"422 refused", http.StatusUnprocessableEntity, `{"status":"refused","reason":"no unresolved terminal failure inside the window on that instance and scope","detail":"nothing was acknowledged and the idempotency key was not consumed"}`, `"refused"`},
		{"core 503", http.StatusServiceUnavailable, `{"error":"Service Unavailable","detail":"collector terminal failure acknowledgement is not configured"}`, "not configured"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: tc.status, body: tc.body}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := authedRequest(http.MethodPost, "/api/v0/admin/collector-failures/acknowledge?probe=1")
			req.Body = io.NopCloser(strings.NewReader(ackPayload))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Fatalf("upstream hits = %d, want exactly 1 (no retry on write-through)", core.hits)
			}
			if core.lastMethod != http.MethodPost {
				t.Errorf("upstream method = %q, want POST", core.lastMethod)
			}
			if core.lastPath != "/api/v0/admin/collector-failures/acknowledge" {
				t.Errorf("upstream path = %q, want the core route", core.lastPath)
			}
			if core.lastQuery != "probe=1" {
				t.Errorf("upstream query = %q, want the caller query forwarded", core.lastQuery)
			}
			if core.lastBody != ackPayload {
				t.Errorf("upstream body = %q, want the caller bytes", core.lastBody)
			}
			if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
				t.Errorf("upstream Authorization = %q, want the forwarded Bearer key", core.lastAuth)
			}
			if rec.Code != tc.status {
				t.Errorf("status = %d, want relayed %d", rec.Code, tc.status)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("Retry-After = %q, want absent (dest never invents the retry signal)", got)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != tc.body {
				t.Errorf("body = %q, want the core bytes %q", body, tc.body)
			}
			if !strings.Contains(string(body), tc.want) {
				t.Errorf("body = %q, want marker %q", body, tc.want)
			}
		})
	}
}

// TestTerminalFailureNative_QueryReadsStillProxy pins that the native
// acknowledge route changes nothing for the family's reads: both query
// POSTs still cross the generic byte-relay with path and method intact.
func TestTerminalFailureNative_QueryReadsStillProxy(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/admin/collector-failures/acknowledgements/query",
		"/api/v0/admin/collector-failures/query",
	} {
		core := &stubCore{status: http.StatusOK, body: `{"count":0,"items":[]}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != path {
			t.Errorf("path %s: hits = %d lastPath = %q, want the generic relay to forward untouched", path, core.hits, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("path %s: status = %d, want relayed 200", path, rec.Code)
		}
	}
}

// TestTerminalFailureNative_UnconfiguredOriginFailsClosed pins that the
// native route fails closed with the permanent 503 when no core origin is
// configured, in both negotiated shapes and both path spellings, without
// dereferencing the unconfigured origin.
func TestTerminalFailureNative_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		for _, path := range []string{
			"/api/v0/admin/collector-failures/acknowledge",
			"/api/v0/admin/collector-failures/acknowledge/",
		} {
			req := authedRequest(http.MethodPost, path)
			if accept != "" {
				req.Header.Set("Accept", accept)
			}
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("Accept %q path %q: panicked with %v, want 503", accept, path, recovered)
					}
				}()
				handler.ServeHTTP(rec, req)
			}()

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("Accept %q path %q: status = %d, want 503", accept, path, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("Accept %q path %q: Retry-After = %q, want absent (permanent 503)", accept, path, got)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), "backend_unavailable") {
				t.Errorf("Accept %q path %q: body = %q, want backend_unavailable", accept, path, body)
			}
		}
	}
}

// TestTerminalFailureNative_BothSpellingsWriteThrough pins that both
// spellings of the acknowledge path stay on the native route: method,
// path (as spelled), and body forward to the same core route.
func TestTerminalFailureNative_BothSpellingsWriteThrough(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/admin/collector-failures/acknowledge",
		"/api/v0/admin/collector-failures/acknowledge/",
	} {
		core := &stubCore{status: http.StatusOK, body: `{"status":"acknowledged","idempotency_key":"k1"}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != path {
			t.Errorf("path %q: hits = %d lastPath = %q, want the native route to forward as spelled", path, core.hits, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("path %q: status = %d, want relayed 200", path, rec.Code)
		}
	}
}

// TestTerminalFailureNative_DeadCoreIs503WithoutRetry pins the dead-core
// shape on the native route: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count proving
// exactly one dispatch attempt (no retry on a write).
func TestTerminalFailureNative_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, "/api/v0/admin/collector-failures/acknowledge")
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
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

// TestTerminalFailureNative_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native acknowledge route: a canceled
// caller gets the permanent 503 backend_unavailable with no retry,
// pre-cancel and mid-flight, with the attempt count proving a single
// dispatch attempt in both cases.
func TestTerminalFailureNative_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
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
			req := authedRequest(http.MethodPost, "/api/v0/admin/collector-failures/acknowledge").WithContext(ctx)
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
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
			}
		})
	}
}

// TestTerminalFailureNative_ParallelProbe pins that concurrent
// acknowledge callers each cross the native route exactly once: N
// parallel requests with distinct keys yield N upstream hits and N
// relayed responses, proving no retry, no drop, and no cross-talk under
// concurrency.
func TestTerminalFailureNative_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"acknowledged"}`)
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
			req := authedRequest(http.MethodPost, "/api/v0/admin/collector-failures/acknowledge")
			req.Body = io.NopCloser(strings.NewReader(
				fmt.Sprintf(`{"collector_kind":"security_alert","instance_id":"i1","scope_id":"s%d","reason":"known outage","idempotency_key":"parallel-%d"}`, i, i)))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				failures[i] = fmt.Sprintf("caller %d: status = %d, want 200", i, rec.Code)
				return
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != `{"status":"acknowledged"}` {
				failures[i] = fmt.Sprintf("caller %d: body = %q, want the relayed core bytes", i, body)
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

// TestTerminalFailureNative_V1AliasOrderingUnchanged pins that the v1
// alias rewrite still lands the acknowledge POST on its native handler
// with the v0 deprecation headers absent.
func TestTerminalFailureNative_V1AliasOrderingUnchanged(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"acknowledged","idempotency_key":"k1"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodPost, "/api/v1/admin/collector-failures/acknowledge")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 || core.lastPath != "/api/v0/admin/collector-failures/acknowledge" {
		t.Errorf("hits = %d path = %q, want alias rewrite to the native v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want relayed 200", rec.Code)
	}
}
