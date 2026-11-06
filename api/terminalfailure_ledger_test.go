// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ledgerEntry is one idempotency-key row of the in-test replay ledger:
// the claimed fingerprint, whether the acknowledgement completed, and
// the stored outcome the duplicate path replays.
type ledgerEntry struct {
	fingerprint string
	completed   bool
	outcome     map[string]any
}

// ledgerCore is a core fake with the replay-ledger semantics the
// acknowledgement route rides on (mirroring the claim/complete pair in
// go/internal/query/admin/acknowledgement/handler.go at the provenance
// pin): a key claims once, a completed key replays its stored outcome
// for the same target, a reused key with a different target is a 409,
// and only the first claim records the effect. It is the ledger half of
// this family PR: the tests below assert the ledger row — not just the
// HTTP bytes — after duplicate deliveries.
type ledgerCore struct {
	t *testing.T

	mu      sync.Mutex
	keys    map[string]*ledgerEntry
	effects int // completed acknowledgements: the one effect a key may record
	hits    int // forwarded POSTs the fake received
}

func newLedgerCore(t *testing.T) *ledgerCore {
	t.Helper()
	return &ledgerCore{t: t, keys: make(map[string]*ledgerEntry)}
}

// fingerprint mirrors the core ackFingerprint scheme at the pin: the
// stable, non-sensitive hash of the acknowledgement target, so a reused
// idempotency key with a different target is rejected rather than treated
// as a duplicate.
func ledgerFingerprint(kind, instance, scope string) string {
	sum := sha256.Sum256([]byte("collector_failure_ack\x00" + kind + "\x00" + instance + "\x00" + scope))
	return hex.EncodeToString(sum[:])
}

func (c *ledgerCore) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.hits++

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			c.t.Errorf("stub read body: %v", err)
		}
		var body struct {
			CollectorKind  string `json:"collector_kind"`
			InstanceID     string `json:"instance_id"`
			ScopeID        string `json:"scope_id"`
			Reason         string `json:"reason"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			writeLedgerError(w, http.StatusBadRequest, "invalid acknowledgement request")
			return
		}
		body.CollectorKind = strings.TrimSpace(body.CollectorKind)
		body.InstanceID = strings.TrimSpace(body.InstanceID)
		body.ScopeID = strings.TrimSpace(body.ScopeID)
		body.Reason = strings.TrimSpace(body.Reason)
		body.IdempotencyKey = strings.TrimSpace(body.IdempotencyKey)
		if body.CollectorKind == "" || body.InstanceID == "" || body.ScopeID == "" {
			writeLedgerError(w, http.StatusBadRequest, "collector_kind, instance_id, and scope_id are required, and reason is at most 512 characters")
			return
		}
		if body.Reason == "" {
			writeLedgerError(w, http.StatusBadRequest, "reason is required and must say why the failure is expected")
			return
		}
		if body.IdempotencyKey == "" {
			writeLedgerError(w, http.StatusBadRequest, "idempotency_key is required to make acknowledgement safe under retries")
			return
		}

		fingerprint := ledgerFingerprint(body.CollectorKind, body.InstanceID, body.ScopeID)
		if existing, ok := c.keys[body.IdempotencyKey]; ok {
			if existing.fingerprint != fingerprint {
				writeLedgerError(w, http.StatusConflict, "idempotency_key was already used with a different collector_kind, instance_id, or scope_id")
				return
			}
			if !existing.completed {
				writeLedgerError(w, http.StatusConflict, "an acknowledgement for this idempotency_key is already in progress")
				return
			}
			writeLedgerJSON(w, http.StatusOK, map[string]any{
				"status":          "duplicate",
				"idempotency_key": body.IdempotencyKey,
				"acknowledgement": existing.outcome,
			})
			return
		}

		// First claim of this key: the one effect. The row completes
		// synchronously in the fake (core completes on a
		// cancel-proof context after the commit); the ledger row the
		// tests assert below is the completed one.
		outcome := map[string]any{
			"collector_kind":    body.CollectorKind,
			"instance_id":       body.InstanceID,
			"scope_id":          body.ScopeID,
			"work_item_id":      "w1",
			"failure_class":     "fetch_error",
			"failure_operation": "fetch",
			"failed_at":         "2026-11-05T00:00:00Z",
			"acknowledged_at":   "2026-11-05T00:01:00Z",
		}
		c.keys[body.IdempotencyKey] = &ledgerEntry{fingerprint: fingerprint, completed: true, outcome: outcome}
		c.effects++
		writeLedgerJSON(w, http.StatusOK, map[string]any{
			"status":          "acknowledged",
			"idempotency_key": body.IdempotencyKey,
			"acknowledgement": outcome,
		})
	})
}

func writeLedgerError(w http.ResponseWriter, status int, detail string) {
	writeLedgerJSON(w, status, map[string]any{"error": http.StatusText(status), "detail": detail})
}

func writeLedgerJSON(w http.ResponseWriter, status int, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// snapshot returns the ledger rows, completed effects, and forwarded hits
// under the fake's lock, for the ledger-state assertions below.
func (c *ledgerCore) snapshot() (map[string]ledgerEntry, int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rows := make(map[string]ledgerEntry, len(c.keys))
	for key, entry := range c.keys {
		rows[key] = *entry
	}
	return rows, c.effects, c.hits
}

// ledgerOutcome decodes one acknowledge-family 200 body into its status
// and acknowledgement object.
func ledgerOutcome(t *testing.T, body []byte) (string, map[string]any) {
	t.Helper()
	var decoded struct {
		Status          string         `json:"status"`
		Acknowledgement map[string]any `json:"acknowledgement"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode acknowledge body %q: %v", body, err)
	}
	return decoded.Status, decoded.Acknowledgement
}

// TestTerminalFailureNative_DuplicateDeliveryCollapsesToOneEffect is the
// center of this family PR: the idempotency-ledger concurrency proof.
// Two identical POSTs cross dest (both forwarded — dest neither retries
// nor dedups), the ledger records exactly one effect, the second answer
// replays the stored outcome, and the asserted ledger row — not just the
// HTTP bytes — is the completed claim for that key and fingerprint.
func TestTerminalFailureNative_DuplicateDeliveryCollapsesToOneEffect(t *testing.T) {
	t.Parallel()

	core := newLedgerCore(t)
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	var firstAck map[string]any
	for i, wantStatus := range []string{"acknowledged", "duplicate"} {
		req := authedRequest(http.MethodPost, "/api/v0/admin/collector-failures/acknowledge")
		req.Body = io.NopCloser(strings.NewReader(ackPayload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("delivery %d: status = %d, want 200", i+1, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		status, ack := ledgerOutcome(t, body)
		if status != wantStatus {
			t.Errorf("delivery %d: status = %q, want %q", i+1, status, wantStatus)
		}
		if i == 0 {
			firstAck = ack
			continue
		}
		if fmt.Sprintf("%v", ack) != fmt.Sprintf("%v", firstAck) {
			t.Errorf("duplicate replay = %v, want the stored outcome %v", ack, firstAck)
		}
	}

	rows, effects, hits := core.snapshot()
	if hits != 2 {
		t.Errorf("upstream hits = %d, want 2 (dest forwards every delivery once: no dest retry, no dest dedup)", hits)
	}
	if effects != 1 {
		t.Errorf("ledger effects = %d, want exactly 1 (duplicate deliveries collapse to one effect)", effects)
	}
	row, ok := rows["k1"]
	if !ok {
		t.Fatalf("ledger lacks key k1: %v", rows)
	}
	if !row.completed {
		t.Errorf("ledger key k1 completed = false, want the completed claim")
	}
	if row.fingerprint != ledgerFingerprint("security_alert", "i1", "s1") {
		t.Errorf("ledger key k1 fingerprint = %q, want the acknowledgement-target fingerprint", row.fingerprint)
	}
	if fmt.Sprintf("%v", row.outcome) != fmt.Sprintf("%v", firstAck) {
		t.Errorf("ledger stored outcome = %v, want the replayed outcome %v", row.outcome, firstAck)
	}
}

// TestTerminalFailureNative_KeyReuseAcrossTargetsIs409 pins the other
// half of the ledger contract: the same key with a different target is a
// 409 that records no new effect, and the original target still replays
// its stored outcome afterwards.
func TestTerminalFailureNative_KeyReuseAcrossTargetsIs409(t *testing.T) {
	t.Parallel()

	core := newLedgerCore(t)
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	post := func(payload string) (int, []byte) {
		req := authedRequest(http.MethodPost, "/api/v0/admin/collector-failures/acknowledge")
		req.Body = io.NopCloser(strings.NewReader(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		body, _ := io.ReadAll(rec.Result().Body)
		return rec.Code, body
	}

	if code, _ := post(ackPayload); code != http.StatusOK {
		t.Fatalf("first delivery status = %d, want 200", code)
	}
	otherTarget := strings.Replace(ackPayload, `"scope_id":"s1"`, `"scope_id":"s2"`, 1)
	code, body := post(otherTarget)
	if code != http.StatusConflict {
		t.Fatalf("cross-target reuse status = %d, want 409", code)
	}
	if !strings.Contains(string(body), "already used with a different") {
		t.Errorf("cross-target reuse body = %q, want the different-target 409", body)
	}
	code, body = post(ackPayload)
	if code != http.StatusOK {
		t.Fatalf("original-target retry status = %d, want 200", code)
	}
	if status, _ := ledgerOutcome(t, body); status != "duplicate" {
		t.Errorf("original-target retry status field = %q, want duplicate", status)
	}

	_, effects, _ := core.snapshot()
	if effects != 1 {
		t.Errorf("ledger effects = %d, want exactly 1 (the 409 recorded nothing)", effects)
	}
}

// TestTerminalFailureNative_ParallelDuplicateDeliveries pins the ledger
// collapse under concurrency: N simultaneous identical POSTs each cross
// dest exactly once, exactly one records the effect, every caller gets a
// 200 (one acknowledged, N-1 duplicate), and the ledger ends completed.
func TestTerminalFailureNative_ParallelDuplicateDeliveries(t *testing.T) {
	t.Parallel()

	core := newLedgerCore(t)
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	const callers = 16
	var wg sync.WaitGroup
	statuses := make([]string, callers)
	failures := make([]string, callers)
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := authedRequest(http.MethodPost, "/api/v0/admin/collector-failures/acknowledge")
			req.Body = io.NopCloser(strings.NewReader(ackPayload))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				failures[i] = fmt.Sprintf("caller %d: status = %d, want 200", i, rec.Code)
				return
			}
			body, _ := io.ReadAll(rec.Result().Body)
			var decoded struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(body, &decoded); err != nil {
				failures[i] = fmt.Sprintf("caller %d: decode %q: %v", i, body, err)
				return
			}
			statuses[i] = decoded.Status
		}(i)
	}
	wg.Wait()

	for _, failure := range failures {
		if failure != "" {
			t.Error(failure)
		}
	}
	var acknowledged, duplicate int
	for _, status := range statuses {
		switch status {
		case "acknowledged":
			acknowledged++
		case "duplicate":
			duplicate++
		default:
			t.Errorf("caller status = %q, want acknowledged or duplicate", status)
		}
	}
	if acknowledged != 1 {
		t.Errorf("acknowledged = %d, want exactly 1 (one effect under concurrency)", acknowledged)
	}
	if duplicate != callers-1 {
		t.Errorf("duplicate = %d, want %d (every other delivery replays)", duplicate, callers-1)
	}

	rows, effects, hits := core.snapshot()
	if hits != callers {
		t.Errorf("upstream hits = %d, want %d (one write-through per caller, no retry)", hits, callers)
	}
	if effects != 1 {
		t.Errorf("ledger effects = %d, want exactly 1", effects)
	}
	row, ok := rows["k1"]
	if !ok || !row.completed {
		t.Errorf("ledger key k1 = %+v, want the completed claim", rows["k1"])
	}
}
