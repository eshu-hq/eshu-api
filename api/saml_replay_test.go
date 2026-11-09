// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// TestSAMLTerm_ParallelDuplicateACSExactlyOnce pins the exactly-once
// replay rule for the ACS mutation: N parallel duplicate deliveries
// of the same RelayState+SAMLResponse make one fetch each (dest holds
// no lock and serializes nothing — core's ConsumeSAMLRequest plus the
// ReserveSAMLReplay reservation judge), the first wins a 201 session
// and every loser relays core's spent-request 401. A dest-side retry
// or dest-side dedup would show as a hit count != N or as two
// winners; a dest-side lock would show in the lane-file hygiene grep
// (no sync/Mutex passes there).
func TestSAMLTerm_ParallelDuplicateACSExactlyOnce(t *testing.T) {
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
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"Unauthorized","detail":"authentication is required"}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	form := "RelayState=opaque-relay& SAMLResponse=opaque-response"
	var created atomic.Int64
	var denied atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			switch rec.Code {
			case http.StatusCreated:
				created.Add(1)
			case http.StatusUnauthorized:
				denied.Add(1)
			default:
				t.Errorf("duplicate ACS status = %d, want 201 (winner) or 401 (loser)", rec.Code)
			}
		}()
	}
	wg.Wait()

	if got := hits.Load(); got != parallel {
		t.Errorf("upstream hits = %d, want %d (one fetch per delivery, no retry, no coalescing)", got, parallel)
	}
	if got := created.Load(); got != 1 {
		t.Errorf("201 winners = %d, want exactly 1 (one session per assertion)", got)
	}
	if got := denied.Load(); got != parallel-1 {
		t.Errorf("401 losers = %d, want %d (loser relays core spent-request error)", got, parallel-1)
	}
}
