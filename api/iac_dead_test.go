// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// W3 IaC-6 dead leg (Refs eshu-hq/eshu-api#102, part of
// eshu-hq/eshu#9669): POST /api/v0/iac/dead terminates ahead of the
// relay on the explicit native route, cleared on the supplemental
// IaCDeadContentAdmission v0 dual pin (eshu-hq/eshu#10642, recorded in
// contracts/iac-read-admission.md).
//
// Dest shape is HTTP write-through on the shared iacWriteThrough
// handler: method, path, query, headers (including the original Bearer
// key), and body forward to the same core route, and core bytes relay
// back unmodified. The materialized branch (reducer rows present,
// repo_name enriched — or silently absent when the ListRepositories
// enrichment read errors) and the derived branch (content_scope page
// from the bounded per-repo file read) are both core behavior, so at
// the dest layer they are canned core answers relayed byte-identical.
// Dest carries no authorizer on this read leg on purpose (api/iac.go):
// dest has no scoped/session resolver, so a dest-side AllScopes gate
// would deny scoped callers the core would allow — core re-judges
// every forwarded call as backstop. The allow-side proof is therefore
// the shared-key pass (not a 401) plus exactly one forward per call;
// the deny side is the gate 401 with zero core contact.

// iacDeadPayload is the representative dead-route request body the
// parity proof forwards: the route always takes an explicit indexed
// IaC scope.
const iacDeadPayload = `{"repo_ids":["repository:r1"],"limit":100}`

// TestIaCDead_AheadOfRelay pins that the dead leg is an explicit
// registration ahead of the generic proxy: a shared-key POST must file
// its dispatch on the native lane (viaNative), not the relay lane,
// with method, path, credential, correlation, and body forwarded
// intact. This test is the swap-mutant detector for dispatch: if the
// registration is removed, the call crosses the generic byte-relay and
// files relay, and this test goes RED.
func TestIaCDead_AheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, spelling := range []string{iacRoutePath(iacDeadRoute), iacRoutePath(iacDeadRoute) + "/"} {
		core := &stubCore{status: http.StatusOK, body: `{"data":{"total":0},"error":null}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		tel := NewTelemetry(nil)
		srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, spelling)
		req.Header.Set("Accept", EnvelopeMIME)
		req.Header.Set("X-Correlation-ID", "iac-dead-ahead")
		req.Body = io.NopCloser(strings.NewReader(iacDeadPayload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%q: status = %d, want 200 from the stub core", spelling, rec.Code)
		}
		if core.hits != 1 {
			t.Fatalf("%q: upstream hits = %d, want exactly 1 (no retry on terminated reads)", spelling, core.hits)
		}
		if core.lastMethod != http.MethodPost {
			t.Errorf("%q: upstream method = %q, want POST", spelling, core.lastMethod)
		}
		if core.lastPath != spelling {
			t.Errorf("%q: upstream path = %q, want the core route as spelled", spelling, core.lastPath)
		}
		if core.lastBody != iacDeadPayload {
			t.Errorf("%q: upstream body = %q, want the caller bytes", spelling, core.lastBody)
		}
		if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
			t.Errorf("%q: upstream Authorization = %q, want the forwarded Bearer key", spelling, core.lastAuth)
		}
		if got := rec.Header().Get("X-Correlation-ID"); got != "iac-dead-ahead" {
			t.Errorf("%q: X-Correlation-ID = %q, want the caller value echoed", spelling, got)
		}
		snap := tel.snapshot()
		if got := snap.dispatches["iac"][viaNative][outcomeCoreOK]; got != 1 {
			t.Errorf("%q: family iac native core_ok = %d, want 1 (registration removed files relay)", spelling, got)
		}
		if lanes := snap.dispatches["iac"]; lanes[viaRelay] != nil {
			t.Errorf("%q: family iac relay lane filed %v, want no relay filing", spelling, lanes[viaRelay])
		}
	}
}

// TestIaCDead_UnauthenticatedIs401 pins that both dead spellings keep
// the dest 401 shape: envelope callers get the envelope error,
// everyone else the plain error_code body, both with the bare Bearer
// challenge — and the core is never reached.
func TestIaCDead_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range []string{iacRoutePath(iacDeadRoute), iacRoutePath(iacDeadRoute) + "/"} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(http.MethodPost, spelling, strings.NewReader(iacDeadPayload))
			if accept != "" {
				req.Header.Set("Accept", accept)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%q Accept %q: status = %d, want 401", spelling, accept, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("%q Accept %q: WWW-Authenticate = %q, want bare Bearer", spelling, accept, got)
			}
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (gate denies before dispatch)", core.hits)
	}
}

// TestIaCDead_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: with no core origin configured both dead spellings answer
// the permanent 503 backend_unavailable instead of panicking, in both
// negotiated shapes.
func TestIaCDead_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range []string{iacRoutePath(iacDeadRoute), iacRoutePath(iacDeadRoute) + "/"} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(http.MethodPost, spelling)
			req.Header.Set("Accept", accept)
			req.Body = io.NopCloser(strings.NewReader(iacDeadPayload))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%q Accept %q: status = %d, want 503", spelling, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%q Accept %q: Retry-After = %q, want none (local 503 is permanent)", spelling, accept, got)
			}
		}
	}
}

// TestIaCDead_RelaysByteForByte pins the termination parity: the
// core's status, headers, and body cross the native dead route
// unmodified — the materialized truth page with repo_name enriched,
// the same page with repo_name silently absent (the ListRepositories
// enrichment read errored core-side), the derived content_scope page,
// the 400 missing-scope body, the 400 selector not-found and ambiguous
// bodies, the scoped-grant 403 (dest adds no gate; the core judges),
// each route-fixed 500 message in its exact core bytes, and the
// retryable 503 keeping the core's Retry-After — with exactly one
// forward attempt per caller.
func TestIaCDead_RelaysByteForByte(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		status  int
		header  http.Header
		body    string
		payload string
	}{
		{
			"materialized truth with repo_name",
			http.StatusOK, nil,
			"{\"data\":{\"analysis_status\":\"materialized_reachability\",\"findings\":[{\"artifact\":\"terraform-modules/modules/orphan-cache\",\"confidence\":0.9,\"evidence\":[\"module directory exists\"],\"family\":\"terraform\",\"finding\":\"candidate_dead_iac\",\"id\":\"dead_iac:terraform:orphan-cache\",\"reachability\":\"unused\",\"repo_id\":\"repository:r1\",\"repo_name\":\"rates\"}],\"findings_count\":1,\"limit\":100,\"limitations\":[\"dynamic templates and variable-selected references are reported as ambiguous\"],\"next_offset\":null,\"offset\":0,\"repo_ids\":[\"repository:r1\"],\"total_findings_count\":1,\"truncated\":false,\"truth_basis\":\"materialized_reducer_rows\"},\"error\":null}\n",
			iacDeadPayload,
		},
		{
			"materialized truth with repo_name silently absent",
			http.StatusOK, nil,
			"{\"data\":{\"analysis_status\":\"materialized_reachability\",\"findings\":[{\"artifact\":\"terraform-modules/modules/orphan-cache\",\"confidence\":0.9,\"evidence\":[\"module directory exists\"],\"family\":\"terraform\",\"finding\":\"candidate_dead_iac\",\"id\":\"dead_iac:terraform:orphan-cache\",\"reachability\":\"unused\",\"repo_id\":\"repository:r1\"}],\"findings_count\":1,\"limit\":100,\"limitations\":[\"dynamic templates and variable-selected references are reported as ambiguous\"],\"next_offset\":null,\"offset\":0,\"repo_ids\":[\"repository:r1\"],\"total_findings_count\":1,\"truncated\":false,\"truth_basis\":\"materialized_reducer_rows\"},\"error\":null}\n",
			iacDeadPayload,
		},
		{
			"derived content_scope truth",
			http.StatusOK, nil,
			"{\"data\":{\"analysis_status\":\"derived_candidate_analysis\",\"findings\":[],\"findings_count\":0,\"limit\":100,\"limitations\":[\"bounded to the requested repository scope\",\"dynamic templates and variable-selected references are reported as ambiguous\",\"exact dead-IaC requires reducer-materialized usage rows\"],\"next_offset\":null,\"offset\":0,\"repo_ids\":[\"repository:r1\"],\"total_findings_count\":0,\"truncated\":false,\"truth_basis\":\"content_scope\"},\"error\":null}\n",
			iacDeadPayload,
		},
		{
			"400 missing repo scope",
			http.StatusBadRequest, nil,
			"{\"detail\":\"repo_id or repo_ids is required\",\"error\":\"Bad Request\"}\n",
			`{}`,
		},
		{
			"400 selector not-found",
			http.StatusBadRequest, nil,
			"{\"detail\":\"repository selector \\\"no-such-repo\\\" did not match any indexed repository\",\"error\":\"Bad Request\"}\n",
			`{"repo_ids":["no-such-repo"],"limit":100}`,
		},
		{
			"400 selector ambiguous",
			http.StatusBadRequest, nil,
			"{\"detail\":\"repository selector \\\"payments-svc\\\" matched multiple repositories: repository:r_a, repository:r_b\",\"error\":\"Bad Request\"}\n",
			`{"repo_ids":["payments-svc"],"limit":100}`,
		},
		{
			"grant denial relays untouched",
			http.StatusForbidden, nil,
			"{\"detail\":\"all-scopes authorization is required\",\"error\":\"Forbidden\"}\n",
			iacDeadPayload,
		},
		{
			"count fixed 500",
			http.StatusInternalServerError, nil,
			"{\"detail\":\"count dead-IaC findings failed\",\"error\":\"Internal Server Error\"}\n",
			iacDeadPayload,
		},
		{
			"list fixed 500",
			http.StatusInternalServerError, nil,
			"{\"detail\":\"list dead-IaC findings failed\",\"error\":\"Internal Server Error\"}\n",
			iacDeadPayload,
		},
		{
			"rows-check fixed 500",
			http.StatusInternalServerError, nil,
			"{\"detail\":\"check dead-IaC materialized rows failed\",\"error\":\"Internal Server Error\"}\n",
			iacDeadPayload,
		},
		{
			"files fixed 500",
			http.StatusInternalServerError, nil,
			"{\"detail\":\"read dead-IaC repository files failed\",\"error\":\"Internal Server Error\"}\n",
			iacDeadPayload,
		},
		{
			"retryable 503 keeps Retry-After",
			http.StatusServiceUnavailable,
			http.Header{"Retry-After": []string{"5"}},
			"{\"detail\":\"IaC inventory database is unavailable\",\"error\":\"Service Unavailable\"}\n",
			iacDeadPayload,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, skeletonTestKey)
				handler := srv.Handler(DefaultV0Sunset)

				target := iacRoutePath(iacDeadRoute)
				req := authedRequest(http.MethodPost, target)
				req.Header.Set("Accept", accept)
				req.Body = io.NopCloser(strings.NewReader(tc.payload))
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Fatalf("%s Accept %q: upstream hits = %d, want exactly 1 (no retry on write-through)", tc.name, accept, core.hits)
				}
				if core.lastMethod != http.MethodPost {
					t.Errorf("%s Accept %q: upstream method = %q, want POST", tc.name, accept, core.lastMethod)
				}
				if core.lastPath != target {
					t.Errorf("%s Accept %q: upstream path = %q, want the core route %q", tc.name, accept, core.lastPath, target)
				}
				if core.lastBody != tc.payload {
					t.Errorf("%s Accept %q: upstream body = %q, want the caller bytes", tc.name, accept, core.lastBody)
				}
				if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
					t.Errorf("%s Accept %q: upstream Authorization = %q, want the forwarded Bearer key", tc.name, accept, core.lastAuth)
				}
				if rec.Code != tc.status {
					t.Errorf("%s Accept %q: status = %d, want relayed %d", tc.name, accept, rec.Code, tc.status)
				}
				if tc.header != nil {
					if got := rec.Header().Get("Retry-After"); got != tc.header.Get("Retry-After") {
						t.Errorf("%s Accept %q: Retry-After = %q, want the core's value relayed", tc.name, accept, got)
					}
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != tc.body {
					t.Errorf("%s Accept %q: body = %q, want the core bytes %q", tc.name, accept, body, tc.body)
				}
			}
		})
	}
}

// TestIaCDead_TrailingSlashStaysNative pins the never-split rule: the
// trailing-slash spelling forwards once to the same core route with
// the spelled path, so the family is never split across dispatch
// paths.
func TestIaCDead_TrailingSlashStaysNative(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	spelling := iacRoutePath(iacDeadRoute) + "/"
	req := authedRequest(http.MethodPost, spelling)
	req.Body = io.NopCloser(strings.NewReader(iacDeadPayload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 || core.lastPath != spelling {
		t.Errorf("hits = %d lastPath = %q, want the native route to forward as spelled", core.hits, core.lastPath)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// TestIaCDead_DeadCoreIs503WithoutRetry pins the dead-core shape on
// the native dead route: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count
// proving exactly one dispatch attempt (no retry on a read).
func TestIaCDead_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, iacRoutePath(iacDeadRoute))
		req.Header.Set("Accept", accept)
		req.Body = io.NopCloser(strings.NewReader(iacDeadPayload))
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

// TestIaCDead_CanceledContextFailsClosedWithoutRetry pins cancellation
// propagation on the native dead route: a canceled caller gets the
// permanent 503 backend_unavailable with no retry, pre-cancel and
// mid-flight, with the attempt count proving a single dispatch attempt
// in both cases.
func TestIaCDead_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
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
			req := authedRequest(http.MethodPost, iacRoutePath(iacDeadRoute)).WithContext(ctx)
			req.Body = io.NopCloser(strings.NewReader(iacDeadPayload))
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
