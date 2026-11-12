// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Code-flow relay-leg fail-closed branches (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#98, W6a). The terminated reader and the
// wave gate are both fail-closed: nil core answers the permanent
// local 503 (never Retry-After), and an empty configured key answers
// the standing 401. The deny shape lives in code_flow_auth_test.go:
// TestCodeFlowDenyBranchShapePinned.

// TestCodeFlowTerminatedAs_NilCore503 pins that the native routes
// with no core origin configured answer the permanent local 503 in
// both negotiated shapes, with zero upstream contact.
func TestCodeFlowTerminatedAs_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)

	for _, path := range codeFlowConcretePaths {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := "POST " + path + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				req := codeFlowAuthedPost(path, accept)
				rec := httptest.NewRecorder()
				srv.serveCodeFlowTerminatedAs(rec, req, true)

				res := rec.Result()
				if res.StatusCode != http.StatusServiceUnavailable {
					t.Fatalf("status = %d, want the permanent local 503", res.StatusCode)
				}
				if res.Header.Get("Retry-After") != "" {
					t.Errorf("Retry-After = %q, want empty (local 503 never carries it)", res.Header.Get("Retry-After"))
				}
			})
		}
	}
}

// TestCodeFlowReadGate_EmptyKey401 pins the wave-gate counterpart:
// an empty configured key answers the standing dest 401 for every
// wave route, with zero core contact.
func TestCodeFlowReadGate_EmptyKey401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, "")

	for _, path := range codeFlowConcretePaths {
		t.Run("POST "+path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(codeFlowParityBody))
			rec := httptest.NewRecorder()
			srv.Handler(DefaultV0Sunset).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing dest 401", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (gate denies with zero core contact)", core.hits)
			}
		})
	}
}
