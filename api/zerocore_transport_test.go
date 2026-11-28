// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

// CoreCallCounter is a counting http.RoundTripper for the zero-core
// proof harness (Refs eshu-hq/eshu-api#449, ADR-0002 section 1). It
// wraps the transport behind Server.httpClient and counts outbound
// core-origin attempts at the Do level, so a retry loop would fail the
// proof even when the core never answers. It records only the request
// method and path: never headers, bodies, or credentials, so counting
// can never leak the bearer key into test output.
//
// The zero value is not usable; build with InstallCoreCallCounter so
// the wrapped base and the client Timeout/CheckRedirect survive.
type CoreCallCounter struct {
	base  http.RoundTripper
	calls atomic.Int64

	mu      sync.Mutex
	methods []string
	paths   []string
}

// RoundTrip implements http.RoundTripper: it records the method and
// path of the call and delegates to the wrapped base transport.
func (c *CoreCallCounter) RoundTrip(req *http.Request) (*http.Response, error) {
	c.calls.Add(1)
	c.mu.Lock()
	c.methods = append(c.methods, req.Method)
	c.paths = append(c.paths, req.URL.Path)
	c.mu.Unlock()
	return c.base.RoundTrip(req)
}

// Calls reports the number of outbound core-origin attempts observed.
func (c *CoreCallCounter) Calls() int {
	return int(c.calls.Load())
}

// AssertZero fails t when any core-origin call was observed. The
// context names the proof step (success or denial path) so a failure
// points at the request that reached for the core.
func (c *CoreCallCounter) AssertZero(t *testing.T, context string) {
	t.Helper()
	if got := c.Calls(); got != 0 {
		t.Fatalf("%s: core-origin calls = %d, want 0 (zero-core violation)", context, got)
	}
}

// InstallCoreCallCounter wraps the EXISTING Server.httpClient transport
// in place with a CoreCallCounter and returns it. A nil transport wraps
// http.DefaultTransport. Timeout and CheckRedirect (see dispatchClient)
// are preserved because the *http.Client itself is never replaced:
// replacing the whole client, as ad-hoc tests do, would silently drop
// the no-redirect rule the relay lane depends on.
func InstallCoreCallCounter(s *Server) *CoreCallCounter {
	base := s.httpClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	counter := &CoreCallCounter{base: base}
	s.httpClient.Transport = counter
	return counter
}

// TestZeroCore_CounterStartsAtZero pins the fresh-counter baseline the
// denial proofs rely on: no request served yet means no call observed.
func TestZeroCore_CounterStartsAtZero(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	counter := InstallCoreCallCounter(srv)
	counter.AssertZero(t, "fresh counter")
}

// TestZeroCore_CounterPreservesClientConfig pins the InstallCoreCallCounter
// contract: the dispatch timeout and the no-redirect rule survive the
// wrap, because the client pointer is reused, not replaced.
func TestZeroCore_CounterPreservesClientConfig(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	beforeTimeout := srv.httpClient.Timeout
	beforeRedirect := srv.httpClient.CheckRedirect != nil
	InstallCoreCallCounter(srv)

	if srv.httpClient.Timeout != beforeTimeout {
		t.Fatalf("Timeout = %v, want %v (client must be reused)", srv.httpClient.Timeout, beforeTimeout)
	}
	if !beforeRedirect || srv.httpClient.CheckRedirect == nil {
		t.Fatal("CheckRedirect lost: relay must never follow core redirects")
	}
	if _, ok := srv.httpClient.Transport.(*CoreCallCounter); !ok {
		t.Fatalf("Transport = %T, want *CoreCallCounter", srv.httpClient.Transport)
	}
}
