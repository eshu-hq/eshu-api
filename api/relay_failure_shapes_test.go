// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Relay failure shapes (eshu-hq/eshu-api#129, part of eshu-hq/eshu#9669
// relay hardening).
//
// TDD RED file: each test below pins the post-fix contract and FAILS on
// the current forwardToCore (api/dispatch.go:116-165). The four shapes:
//
// (a) refused connection: 503 backend_unavailable, no Retry-After, and no
// reason naming the failure (dispatch.go:137 fail path).
// (b) pre-header client timeout: the SAME 503 body as (a), so callers and
// operators cannot tell refused from timeout apart (dispatch.go:137).
// (c) post-header stall: status 200 already written (:163), then io.Copy
// hits the client timeout and the error is discarded (:164) — a real
// client reads a clean 200 end with a body cut off mid-JSON, and the
// outcome stays core_ok.
// (d) mid-copy core error: the copy error is discarded (:164) — same
// clean truncated 200 as (c), with nothing logged.
//
// The reason vocabulary below follows the issue's recommended option
// (details.reason: connection refused, timeout, other) pending the
// arbiter decision on pre-header 503-vs-504. All four tests use a real
// socket on the relay side so the mid-stream shapes observe what a real
// caller sees; httptest.ResponseRecorder cannot show a clean end versus
// an aborted connection.

// relayTestClient issues relay calls with a caller-side bound far above
// the relay's own dispatch bound, so a stall surfaces relay behavior,
// not caller impatience.
func relayTestClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// relayGet performs one authed relay GET over a real socket.
func relayGet(t *testing.T, relayURL, path string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, relayURL+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	return relayTestClient().Do(req)
}

// TestRelayFailureShapes_RefusedConnectionNamesReason pins shape (a):
// a refused core connection keeps the standing fail-closed 503 with no
// Retry-After, and names the reason so it is distinguishable from a
// pre-header timeout. RED today: the body carries no reason.
func TestRelayFailureShapes_RefusedConnectionNamesReason(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	relay := httptest.NewServer(srv.Handler(DefaultV0Sunset))
	defer relay.Close()

	resp, err := relayGet(t, relay.URL, "/api/v0/repositories")
	if err != nil {
		t.Fatalf("relay call: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (standing fail-closed contract)", resp.StatusCode)
	}
	if got := resp.Header.Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "backend_unavailable") {
		t.Errorf("body = %q, want backend_unavailable", body)
	}
	if !strings.Contains(string(body), `"reason":"connection_refused"`) {
		t.Errorf("body = %q, want details.reason connection_refused distinguishing refused from timeout", body)
	}
}

// TestRelayFailureShapes_PreHeaderTimeoutNamesReason pins shape (b): a
// core that never answers before the dispatch bound keeps 503, and names
// the timeout reason so it differs from shape (a). RED today: the body
// is byte-identical to the refused-connection 503.
func TestRelayFailureShapes_PreHeaderTimeoutNamesReason(t *testing.T) {
	t.Parallel()

	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer core.Close()

	srv := NewServer(core.URL, skeletonTestKey)
	bound := dispatchClient()
	bound.Timeout = 200 * time.Millisecond
	srv.httpClient = bound
	relay := httptest.NewServer(srv.Handler(DefaultV0Sunset))
	defer relay.Close()

	resp, err := relayGet(t, relay.URL, "/api/v0/repositories")
	if err != nil {
		t.Fatalf("relay call: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 after the dispatch bound", resp.StatusCode)
	}
	if got := resp.Header.Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "backend_unavailable") {
		t.Errorf("body = %q, want backend_unavailable", body)
	}
	if !strings.Contains(string(body), `"reason":"timeout"`) {
		t.Errorf("body = %q, want details.reason timeout distinguishing timeout from refused", body)
	}
}

// TestRelayFailureShapes_RefusedEnvelopeNamesReason pins the envelope
// spelling of shape (a): the negotiated envelope 503 carries the same
// details.reason as the plain shape, with no Retry-After.
func TestRelayFailureShapes_RefusedEnvelopeNamesReason(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	relay := httptest.NewServer(srv.Handler(DefaultV0Sunset))
	defer relay.Close()

	req, err := http.NewRequest(http.MethodGet, relay.URL+"/api/v0/repositories", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	req.Header.Set("Accept", EnvelopeMIME)
	resp, err := relayTestClient().Do(req)
	if err != nil {
		t.Fatalf("relay call: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (standing fail-closed contract)", resp.StatusCode)
	}
	if got := resp.Header.Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), `"code":"backend_unavailable"`) {
		t.Errorf("body = %q, want envelope code backend_unavailable", body)
	}
	if !strings.Contains(string(body), `"reason":"connection_refused"`) {
		t.Errorf("body = %q, want details.reason connection_refused in the envelope shape", body)
	}
}

// TestRelayFailureShapes_FailureLogFieldDistinguishes pins the operator
// signal: the structured api-request line carries the closed failure
// field, so refused reads apart from timeout without touching the
// outcome vocabulary the dispatch counters assert.
func TestRelayFailureShapes_FailureLogFieldDistinguishes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		origin string
		reason string
	}{
		{"refused", "http://127.0.0.1:1", "connection_refused"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var logs strings.Builder
			tel := NewTelemetry(slog.New(slog.NewJSONHandler(&logs, nil)))
			srv := NewServer(tc.origin, skeletonTestKey)
			srv.telemetry = tel
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, authedRequest("GET", "/api/v0/repositories"))
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
			lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
			if len(lines) != 1 {
				t.Fatalf("log lines = %d, want 1 structured line", len(lines))
			}
			if !strings.Contains(lines[0], `"failure":"`+tc.reason+`"`) {
				t.Errorf("log line = %s, want failure %q", lines[0], tc.reason)
			}
			if !strings.Contains(lines[0], `"outcome":"local_503"`) {
				t.Errorf("log line = %s, want the standing outcome local_503 beside failure", lines[0])
			}
		})
	}
}

// TestRelayFailureShapes_PostHeaderStallAborts pins shape (c): a core
// that answers headers plus a partial body, then stalls past the dispatch
// bound, must NOT yield a clean 200 end — the relay aborts the copy so
// the caller sees a transport error instead of truncated JSON. RED
// today: the caller reads a clean EOF with the body cut off mid-JSON.
func TestRelayFailureShapes_PostHeaderStallAborts(t *testing.T) {
	t.Parallel()

	const partial = `{"data":[`
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, partial)
		w.(http.Flusher).Flush()
		time.Sleep(3 * time.Second)
	}))
	defer core.Close()

	srv := NewServer(core.URL, skeletonTestKey)
	bound := dispatchClient()
	bound.Timeout = 300 * time.Millisecond
	srv.httpClient = bound
	relay := httptest.NewServer(srv.Handler(DefaultV0Sunset))
	defer relay.Close()

	resp, err := relayGet(t, relay.URL, "/api/v0/repositories")
	if err != nil {
		t.Fatalf("relay call: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want the already-written 200 (headers precede the stall)", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err == nil {
		t.Errorf("body read ended clean with %q, want a transport error: 200 with truncated JSON looks complete", body)
	}
	if string(body) == partial {
		t.Logf("observed current shape: clean 200 with body cut off at %q", body)
	}
}

// TestRelayFailureShapes_GetWithBodyRelaysUntouched pins the request-body
// half of the GET with/without body box: a GET carrying a body forwards
// the bytes to the core (the echo proves the upstream saw them) and the
// core's answer relays byte-for-byte. Every other test in this file uses
// a bodyless GET; without this pin a regression that dropped GET bodies
// would stay silent.
func TestRelayFailureShapes_GetWithBodyRelaysUntouched(t *testing.T) {
	t.Parallel()

	const payload = `{"q":"body-on-get"}`
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, r.Body)
	}))
	defer core.Close()

	srv := NewServer(core.URL, skeletonTestKey)
	relay := httptest.NewServer(srv.Handler(DefaultV0Sunset))
	defer relay.Close()

	req, err := http.NewRequest(http.MethodGet, relay.URL+"/api/v0/repositories", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	resp, err := relayTestClient().Do(req)
	if err != nil {
		t.Fatalf("relay call: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != payload {
		t.Errorf("body = %q, want the echoed bytes %q (GET body forwarded, answer relayed)", body, payload)
	}
}

// TestRelayFailureShapes_MidCopyErrorAborts pins shape (d): a core that
// declares more bytes than it sends (headers, then a short close) fails
// the relay copy, and that failure must NOT end clean — the caller sees
// a transport error instead of truncated JSON. RED today: the copy
// error is discarded and the caller reads a clean 200 end.
func TestRelayFailureShapes_MidCopyErrorAborts(t *testing.T) {
	t.Parallel()

	const partial = `{"data":[1,2`
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, partial)
	}))
	defer core.Close()

	srv := NewServer(core.URL, skeletonTestKey)
	relay := httptest.NewServer(srv.Handler(DefaultV0Sunset))
	defer relay.Close()

	resp, err := relayGet(t, relay.URL, "/api/v0/repositories")
	if err != nil {
		t.Fatalf("relay call: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want the already-written 200 (headers precede the short close)", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err == nil {
		t.Errorf("body read ended clean with %q, want a transport error: the mid-copy failure is discarded today", body)
	}
}
