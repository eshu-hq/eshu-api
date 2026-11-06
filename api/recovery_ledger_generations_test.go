// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
)

func (c *recoveryLedgerCore) serveRecoverGenerations(w http.ResponseWriter, body map[string]any) {
	scopeIDsRaw, _ := body["scope_ids"].([]any)
	allScopes, _ := body["all_scopes"].(bool)
	reason, _ := body["reason"].(string)
	key, _ := body["idempotency_key"].(string)

	var scopeIDs []string
	for _, id := range scopeIDsRaw {
		scopeIDs = append(scopeIDs, fmt.Sprintf("%v", id))
	}
	sort.Strings(scopeIDs)
	if allScopes && len(scopeIDs) > 0 {
		writeRecoveryLedgerError(w, http.StatusBadRequest, "all_scopes cannot be combined with scope_ids")
		return
	}
	if !allScopes && len(scopeIDs) == 0 {
		writeRecoveryLedgerError(w, http.StatusBadRequest, "scope_ids is required and must name at least one wedged scope")
		return
	}
	if reason == "" {
		writeRecoveryLedgerError(w, http.StatusBadRequest, "reason is required and must explain why the recovery is safe")
		return
	}
	if key == "" {
		writeRecoveryLedgerError(w, http.StatusBadRequest, "idempotency_key is required to make recovery safe under retries")
		return
	}

	fingerprint := recoveryFingerprint(append(scopeIDs, fmt.Sprint(allScopes))...)
	effect := map[string]any{
		"status": "recovered", "enqueued": len(scopeIDs),
		"scope_ids":       scopeIDs,
		"idempotency_key": key, "duplicate": false,
	}
	claimed, outcome, status, detail := c.claim(key, fingerprint, effect)
	if status != http.StatusOK {
		writeRecoveryLedgerError(w, status, detail)
		return
	}
	if !claimed {
		outcome = map[string]any{
			"status": "recovered", "enqueued": outcome["enqueued"],
			"scope_ids":       outcome["scope_ids"],
			"idempotency_key": key, "duplicate": true,
		}
	}
	writeRecoveryLedgerJSON(w, http.StatusOK, outcome)
}

func (c *recoveryLedgerCore) serveRefinalize(w http.ResponseWriter, body map[string]any) {
	scopeIDs, _ := body["scope_ids"].([]any)
	if len(scopeIDs) == 0 {
		writeRecoveryLedgerError(w, http.StatusBadRequest, "scope_ids is required and must not be empty")
		return
	}
	// Refinalize carries no idempotency key and records no ledger row;
	// every accepted call is an effect.
	c.effects++
	writeRecoveryLedgerJSON(w, http.StatusOK, map[string]any{
		"status": "accepted", "enqueued": 1, "scope_ids": scopeIDs,
	})
}

// seedInProgress plants a stranded in_progress row for key, mirroring a
// replay whose mutation committed but whose finalize never ran: the
// next delivery with the same key must answer 409, not replay.
func TestRecoveryNative_RecoverGenerationsLedgerProtocol(t *testing.T) {
	t.Parallel()

	core := newRecoveryLedgerCore(t)
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	list := `{"scope_ids":["s1"],"reason":"rebuild after restore","idempotency_key":"k-rg"}`
	if code, decoded := postRecovery(t, handler, "/api/v0/admin/recover-generations", list); code != http.StatusOK || decoded["duplicate"] == true {
		t.Fatalf("first delivery: status = %d duplicate = %v, want 200/false", code, decoded["duplicate"])
	}
	if code, decoded := postRecovery(t, handler, "/api/v0/admin/recover-generations", list); code != http.StatusOK || decoded["duplicate"] != true {
		t.Fatalf("duplicate delivery: status = %d duplicate = %v, want 200/true", code, decoded["duplicate"])
	}
	if code, _ := postRecovery(t, handler, "/api/v0/admin/recover-generations",
		`{"all_scopes":true,"reason":"rebuild after restore","idempotency_key":"k-rg"}`); code != http.StatusConflict {
		t.Fatalf("mode reuse: status = %d, want 409", code)
	}

	_, effects, hits := core.snapshot()
	if hits != 3 {
		t.Errorf("upstream hits = %d, want 3 (dest forwards every delivery once)", hits)
	}
	if effects != 1 {
		t.Errorf("ledger effects = %d, want exactly 1 (duplicate and 409 record nothing)", effects)
	}
}

// TestRecoveryNative_ParallelDuplicateDeliveries pins the concurrent
// half of the ledger contract: 16 parallel identical replay deliveries
// forward once each, record exactly one effect, and all answer 200
// (first or stored-outcome duplicate).
func TestRecoveryNative_ParallelDuplicateDeliveries(t *testing.T) {
	t.Parallel()

	core := newRecoveryLedgerCore(t)
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	const callers = 16
	payload := `{"scope_id":"s1","reason":"retry after the fix deployed","idempotency_key":"k-par"}`
	codes := make([]int, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code, _ := postRecovery(t, handler, "/api/v0/admin/replay", payload)
			codes[i] = code
		}(i)
	}
	wg.Wait()

	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("caller %d: status = %d, want 200", i, code)
		}
	}
	_, effects, hits := core.snapshot()
	if hits != callers {
		t.Errorf("upstream hits = %d, want %d (one forward per caller)", hits, callers)
	}
	if effects != 1 {
		t.Errorf("ledger effects = %d, want exactly 1 (parallel duplicates collapse to one effect)", effects)
	}
}
