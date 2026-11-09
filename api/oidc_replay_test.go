// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// TestOIDCTerm_ParallelDuplicateCallbackExactlyOnce pins the
// exactly-once replay rule for the callback mutation: N parallel
// duplicate deliveries of the same state+code make one fetch each
// (dest holds no lock and serializes nothing — core's ConsumeState
// judges), the first wins a 201 session and every loser relays
// core's 403. A dest-side retry or dest-side dedup would show as a
// hit count != N or as two winners; a dest-side lock would show in
// the lane-file hygiene grep (no sync/Mutex passes there).
func TestOIDCTerm_ParallelDuplicateCallbackExactlyOnce(t *testing.T) {
	t.Parallel()

	const parallel = 16
	var hits atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			w.Header().Add("Set-Cookie", "__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict")
			w.Header().Add("Set-Cookie", "__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"csrf_token":"opaque-csrf"}`)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":"Forbidden","detail":"oidc login denied"}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	var created atomic.Int64
	var denied atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/callback?state=st-opaque-123&code=cd-opaque-456", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			switch rec.Code {
			case http.StatusCreated:
				created.Add(1)
			case http.StatusForbidden:
				denied.Add(1)
			default:
				t.Errorf("duplicate callback status = %d, want 201 (winner) or 403 (loser)", rec.Code)
			}
		}()
	}
	wg.Wait()

	if got := hits.Load(); got != parallel {
		t.Errorf("upstream hits = %d, want %d (one fetch per delivery, no retry, no coalescing)", got, parallel)
	}
	if got := created.Load(); got != 1 {
		t.Errorf("201 winners = %d, want exactly 1 (one session per state)", got)
	}
	if got := denied.Load(); got != parallel-1 {
		t.Errorf("403 losers = %d, want %d (loser relays core error)", got, parallel-1)
	}
}
