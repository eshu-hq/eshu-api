// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// W8a fail-closed branches (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#100). The zero-hit blocks in w8a_terminate.go are
// all fail-closed guards: nil core origin and empty configured key
// must answer locally without touching core. These tests pin those
// guards and the unreachable deny branch so the coverage floor holds
// as the wave grows.

// TestW8aTerminatedAs_NilCore503 pins that a native route with no
// core origin answers permanent 503 with zero core contact.
func TestW8aTerminatedAs_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	req := httptest.NewRequest(http.MethodPost, "/api/v0/search/semantic", nil)
	rec := httptest.NewRecorder()
	srv.serveW8aTerminatedAs(rec, req, true)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil core status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if rec.Header().Get("Retry-After") != "" {
		t.Errorf("nil core Retry-After = %q, want empty (permanent, never retry)", rec.Header().Get("Retry-After"))
	}
}

// TestW8aTerminatedAs_Deny403 pins the unreachable deny branch: with
// allScopes false the terminated reader files the denied outcome on
// the native lane and never dispatches, even with a core configured
// (no request leaves dest).
func TestW8aTerminatedAs_Deny403(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/status/governance", nil)
	rec := httptest.NewRecorder()
	srv.serveW8aTerminatedAs(rec, req, false)
	if rec.Code != http.StatusForbidden {
		t.Errorf("deny status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestW8aReadGate_EmptyKey401 pins that an empty configured key fails
// closed like RequireAuth: every request gets the standing 401 with
// zero core contact.
func TestW8aReadGate_EmptyKey401(t *testing.T) {
	t.Parallel()

	srv := NewServer("", "")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler ran with empty configured key")
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v0/metrics/timeseries", nil)
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	rec := httptest.NewRecorder()
	srv.w8aReadGate(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("empty key status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestW8aRelayExplicit_NilCore503 pins that the explicit relay with
// no core origin answers permanent 503 with zero core contact.
func TestW8aRelayExplicit_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/secrets-iam/posture-summary", nil)
	rec := httptest.NewRecorder()
	srv.w8aRelayExplicit(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil core relay status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if rec.Header().Get("Retry-After") != "" {
		t.Errorf("nil core relay Retry-After = %q, want empty (permanent, never retry)", rec.Header().Get("Retry-After"))
	}
}
