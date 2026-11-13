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

// TestIaCDead_ParallelProbe pins that concurrent callers each get one
// write-through: 16 parallel POSTs on the dead leg (both spellings)
// answer 200 with the relayed core bytes and land 16 upstream hits (one
// forward per caller, no dest coalescing and no dest retry).
func TestIaCDead_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":{"total":0},"error":null}`)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	paths := []string{
		iacRoutePath(iacDeadRoute),
		iacRoutePath(iacDeadRoute) + "/",
	}
	const callers = 16
	var wg sync.WaitGroup
	failures := make([]string, callers)
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := authedRequest(http.MethodPost, paths[i%len(paths)])
			req.Body = io.NopCloser(strings.NewReader(iacDeadPayload))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				failures[i] = paths[i%len(paths)]
			}
		}(i)
	}
	wg.Wait()

	for i, path := range failures {
		if path != "" {
			t.Errorf("caller %d %q: non-200 answer", i, path)
		}
	}
	if got := hits.Load(); got != callers {
		t.Errorf("upstream hits = %d, want %d (one forward per caller)", got, callers)
	}
}

// TestIaCDeadNative_MatchesGenericRelay is the revert-to-relay proof:
// the identical stub-core answer crosses the dead native route and the
// generic relay with byte-identical status and body. If the native
// registration is removed, the native leg files relay and the
// lane-filing assertions below fail.
func TestIaCDeadNative_MatchesGenericRelay(t *testing.T) {
	t.Parallel()

	const payload = `{"repo_ids":["repository:r1"],"limit":100}`
	const stubBody = "{\"data\":{\"total\":1},\"error\":null}\n"

	nativeCore := &stubCore{status: http.StatusOK, body: stubBody}
	nativeMux := http.NewServeMux()
	nativeMux.Handle("/", nativeCore.handler())
	nativeUpstream := httptest.NewServer(nativeMux)
	defer nativeUpstream.Close()

	relayCore := &stubCore{status: http.StatusOK, body: stubBody}
	relayMux := http.NewServeMux()
	relayMux.Handle("/", relayCore.handler())
	relayUpstream := httptest.NewServer(relayMux)
	defer relayUpstream.Close()

	nativeTel := NewTelemetry(nil)
	nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey).WithTelemetry(nativeTel)
	nativeRec := httptest.NewRecorder()
	nativeReq := authedRequest(http.MethodPost, iacRoutePath(iacDeadRoute))
	nativeReq.Body = io.NopCloser(strings.NewReader(payload))
	nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, nativeReq)

	relayTel := NewTelemetry(nil)
	relaySrv := NewServer(relayUpstream.URL, skeletonTestKey).WithTelemetry(relayTel)
	relayRec := httptest.NewRecorder()
	relayReq := authedRequest(http.MethodPost, iacRoutePath(iacDeadRoute))
	relayReq.Body = io.NopCloser(strings.NewReader(payload))
	relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, relayReq)

	nativeBody, _ := io.ReadAll(nativeRec.Result().Body)
	relayBody, _ := io.ReadAll(relayRec.Result().Body)
	if nativeRec.Code != relayRec.Code || string(nativeBody) != string(relayBody) {
		t.Fatalf("native = (%d, %q), relay = (%d, %q): want byte-identical", nativeRec.Code, nativeBody, relayRec.Code, relayBody)
	}
	if got := nativeTel.snapshot().dispatches["iac"][viaNative][outcomeCoreOK]; got != 1 {
		t.Errorf("native iac core_ok = %d, want 1", got)
	}
	if got := relayTel.snapshot().dispatches["iac"][viaRelay][outcomeCoreOK]; got != 1 {
		t.Errorf("relay iac core_ok = %d, want 1", got)
	}
}
