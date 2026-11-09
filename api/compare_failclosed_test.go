// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Compare relay-leg fail-closed branches (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72, gap 04). The three zero-hit blocks
// in compare_terminate.go are all fail-closed guards: nil core
// origin and empty configured key must answer locally without
// touching core. These tests pin those guards so the coverage floor
// holds as the wave grows. The unreachable deny branch rides
// TestCompareDenyBranchShapePinned in compare_auth_test.go.

// TestCompareTerminatedAs_NilCore503 pins that the native route with
// no core origin answers permanent 503 with zero core contact.
func TestCompareTerminatedAs_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	req := httptest.NewRequest(http.MethodPost, compareConcretePath, strings.NewReader(compareParityBody))
	rec := httptest.NewRecorder()
	srv.serveCompareTerminatedAs(rec, req, true)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil core status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if rec.Header().Get("Retry-After") != "" {
		t.Errorf("nil core Retry-After = %q, want empty (permanent, never retry)", rec.Header().Get("Retry-After"))
	}
}

// TestCompareReadGate_EmptyKey401 pins that an empty configured key
// fails closed like RequireAuth: every request gets the standing 401
// with zero core contact.
func TestCompareReadGate_EmptyKey401(t *testing.T) {
	t.Parallel()

	srv := NewServer("", "")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler ran with empty configured key")
	})
	req := httptest.NewRequest(http.MethodPost, compareConcretePath, strings.NewReader(compareParityBody))
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	rec := httptest.NewRecorder()
	srv.compareReadGate(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("empty key status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestCompareRelayExplicit_NilCore503 pins that the explicit relay
// with no core origin answers permanent 503 with zero core contact.
func TestCompareRelayExplicit_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	req := httptest.NewRequest(http.MethodPost, compareConcretePath, strings.NewReader(compareParityBody))
	rec := httptest.NewRecorder()
	srv.compareRelayExplicit(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil core relay status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if rec.Header().Get("Retry-After") != "" {
		t.Errorf("nil core relay Retry-After = %q, want empty (permanent, never retry)", rec.Header().Get("Retry-After"))
	}
}
