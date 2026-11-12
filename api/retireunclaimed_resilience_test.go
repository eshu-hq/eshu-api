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

// TestRetireUnclaimedNative_DeadCoreIs503WithoutRetry pins the dead-core
// shape on the native route: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count proving
// exactly one dispatch attempt (no retry on a write).
func TestRetireUnclaimedNative_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed")
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
		if !strings.Contains(string(body), backendUnavailableCode) {
			t.Errorf("Accept %q: body = %q, want backend_unavailable", accept, body)
		}
	}
}

// TestRetireUnclaimedNative_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native route: a canceled caller gets
// the permanent 503 backend_unavailable with no retry, pre-cancel and
// mid-flight, with the attempt count proving a single dispatch attempt
// in both cases.
func TestRetireUnclaimedNative_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
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
			req := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed").WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := atomic.LoadInt32(&attempts); got != 1 {
				t.Errorf("dispatch attempts = %d, want exactly 1 (no retry)", got)
			}
			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), backendUnavailableCode) {
				t.Errorf("body = %q, want backend_unavailable", body)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
			}
		})
	}
}

// TestRetireUnclaimedNative_ParallelProbe pins that concurrent POST
// callers each cross the native route exactly once: N parallel requests
// yield N upstream hits and N relayed responses, proving no retry, no
// drop, and no cross-talk under concurrency.
func TestRetireUnclaimedNative_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"count":0,"items":[]}`)
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
			req := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				failures[i] = fmt.Sprintf("caller %d: status = %d, want 200", i, rec.Code)
				return
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != `{"count":0,"items":[]}` {
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

// TestRetireUnclaimedNative_SequentialDuplicateIsNoWorseThanCore pins
// the state-idempotent duplicate posture the pin freezes: a duplicate
// submit executes again instead of replaying a stored outcome. The core
// answers the first delivery with the transitioned rows and the second
// with the residue (or a zero count); dest forwards twice and relays
// both core answers byte-for-byte. Two caller calls yield exactly two
// upstream hits with no dest-side collapse, synthesis, or ledger —
// byte-parity on the duplicate is no-worse-than-core by construction.
func TestRetireUnclaimedNative_SequentialDuplicateIsNoWorseThanCore(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n := hits.Add(1); n == 1 {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"count":2,"items":[{"work_item_id":"w-1","scope_id":"s-1"},{"work_item_id":"w-2","scope_id":"s-2"}]}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"count":0,"items":[]}`)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	post := func() (int, string) {
		req := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed")
		req.Body = io.NopCloser(strings.NewReader(retireUnclaimedPayload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		body, _ := io.ReadAll(rec.Result().Body)
		return rec.Code, string(body)
	}

	firstCode, firstBody := post()
	if firstCode != http.StatusOK {
		t.Fatalf("first delivery: status = %d, want 200", firstCode)
	}
	if firstBody != `{"count":2,"items":[{"work_item_id":"w-1","scope_id":"s-1"},{"work_item_id":"w-2","scope_id":"s-2"}]}` {
		t.Fatalf("first delivery body = %q, want the relayed core bytes", firstBody)
	}

	secondCode, secondBody := post()
	if secondCode != http.StatusOK {
		t.Fatalf("duplicate delivery: status = %d, want 200", secondCode)
	}
	if secondBody != `{"count":0,"items":[]}` {
		t.Fatalf("duplicate delivery body = %q, want the relayed residue/zero (never a replayed outcome)", secondBody)
	}

	if got := hits.Load(); got != 2 {
		t.Errorf("upstream hits = %d, want 2 (the duplicate executes again; dest collapses nothing)", got)
	}
}

// TestRetireUnclaimedNative_RevertToRelayRehearsal proves the rollback
// the contract records: with the native registration absent, the same
// POST crosses the generic byte-relay with the same wire contract — no
// core change, no migration, no flag. The rehearsal serves the payload
// through a relay-only mux (the generic dispatch the routes table falls
// back to when this lane's commit reverts) and requires identical
// status and bytes to the native route for the same core answer.
func TestRetireUnclaimedNative_RevertToRelayRehearsal(t *testing.T) {
	t.Parallel()

	const coreBody = `{"count":1,"items":[{"work_item_id":"w-9","scope_id":"s-9"}]}`
	core := &stubCore{status: http.StatusOK, body: coreBody}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)

	native := srv.Handler(DefaultV0Sunset)
	nativeReq := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed")
	nativeReq.Body = io.NopCloser(strings.NewReader(retireUnclaimedPayload))
	nativeRec := httptest.NewRecorder()
	native.ServeHTTP(nativeRec, nativeReq)
	if core.hits != 1 {
		t.Fatalf("native: upstream hits = %d, want 1", core.hits)
	}

	relayOnly := http.NewServeMux()
	relayOnly.Handle("/api/", srv.instrumented(viaRelay, RequireAuth(srv.apiKey, http.HandlerFunc(srv.dispatch))))
	relayReq := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed")
	relayReq.Body = io.NopCloser(strings.NewReader(retireUnclaimedPayload))
	relayRec := httptest.NewRecorder()
	relayOnly.ServeHTTP(relayRec, relayReq)

	if core.hits != 2 {
		t.Fatalf("relay: upstream hits = %d, want 2 (relay still forwards the route)", core.hits)
	}
	if relayRec.Code != nativeRec.Code {
		t.Errorf("relay status = %d, native status = %d: revert must keep the wire contract", relayRec.Code, nativeRec.Code)
	}
	relayBody, _ := io.ReadAll(relayRec.Result().Body)
	nativeBody, _ := io.ReadAll(nativeRec.Result().Body)
	if string(relayBody) != string(nativeBody) || string(relayBody) != coreBody {
		t.Errorf("relay body = %q, native body = %q, want both %q", relayBody, nativeBody, coreBody)
	}
	if core.lastPath != "/api/v0/admin/workflow-retire-unclaimed" {
		t.Errorf("relay upstream path = %q, want the core route", core.lastPath)
	}
}
