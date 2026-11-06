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
	"sort"
	"strings"
	"sync"
	"testing"
)

// recoveryLedgerEntry is one idempotency-key row of the in-test ledger:
// the claimed fingerprint, whether the mutation completed, and the
// stored outcome the duplicate path replays.
type recoveryLedgerEntry struct {
	fingerprint string
	completed   bool
	outcome     map[string]any
}

// recoveryLedgerCore is a core fake with the replay-ledger semantics
// the recovery routes ride on (mirroring the claim/complete pair in
// go/internal/query/admin at the provenance pin: replay.go,
// reopen.go, and generations.go claim the admin_replay_requests row
// before the mutation and complete it after, on a cancel-proof
// context): a key claims once, a completed key replays its stored
// outcome for the same target, a reused key with a different target is
// a 409, an in-progress key is a 409, and only the first claim records
// the effect. Refusals that run before the claim (unknown scope,
// unsafe class, bad mode mix) consume nothing. Refinalize is the
// exception: it carries no idempotency key and records no ledger row.
// It is the ledger half of this family PR: the tests below assert the
// ledger row — not just the HTTP bytes — after duplicate deliveries.
type recoveryLedgerCore struct {
	t *testing.T

	mu      sync.Mutex
	keys    map[string]*recoveryLedgerEntry
	effects int // completed mutations: the one effect a key may record
	hits    int // forwarded POSTs the fake received
}

func newRecoveryLedgerCore(t *testing.T) *recoveryLedgerCore {
	t.Helper()
	return &recoveryLedgerCore{t: t, keys: make(map[string]*recoveryLedgerEntry)}
}

// recoveryFingerprint hashes the normalized mutation target, mirroring
// the core fingerprint scheme at the pin: the same target reproduces
// the fingerprint (duplicate), any target change does not (409). The
// exact core construction is core-owned; the fake keeps the same
// contract — equality on the same target, inequality across targets.
func recoveryFingerprint(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func writeRecoveryLedgerError(w http.ResponseWriter, status int, detail string) {
	raw, err := json.Marshal(map[string]any{"error": http.StatusText(status), "detail": detail})
	if err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

func writeRecoveryLedgerJSON(w http.ResponseWriter, status int, value map[string]any) {
	raw, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// claim applies the shared ledger protocol: first claim of a key
// records the pending row (completed synchronously below, as core does
// on its cancel-proof context); a duplicate claim of the same target
// replays the stored outcome; a reused key with a different target, or
// a still-in-progress key, is a 409.
func (c *recoveryLedgerCore) claim(key, fingerprint string, effect map[string]any) (claimed bool, outcome map[string]any, status int, detail string) {
	if existing, ok := c.keys[key]; ok {
		if existing.fingerprint != fingerprint {
			return false, nil, http.StatusConflict, "idempotency_key was already used with different parameters"
		}
		if !existing.completed {
			return false, nil, http.StatusConflict, "a mutation for this idempotency_key is already in progress"
		}
		return false, existing.outcome, http.StatusOK, ""
	}
	c.keys[key] = &recoveryLedgerEntry{fingerprint: fingerprint, completed: true, outcome: effect}
	c.effects++
	return true, effect, http.StatusOK, ""
}

func (c *recoveryLedgerCore) serveReplay(w http.ResponseWriter, body map[string]any) {
	ids, _ := body["work_item_ids"].([]any)
	scopeID, _ := body["scope_id"].(string)
	stage, _ := body["stage"].(string)
	failureClass, _ := body["failure_class"].(string)
	reason, _ := body["reason"].(string)
	key, _ := body["idempotency_key"].(string)
	force, _ := body["force"].(bool)
	limit, _ := body["limit"].(float64)

	if len(ids) == 0 && scopeID == "" && stage == "" && failureClass == "" {
		writeRecoveryLedgerError(w, http.StatusBadRequest, "at least one selector is required")
		return
	}
	if reason == "" {
		writeRecoveryLedgerError(w, http.StatusBadRequest, "reason is required and must explain why the replay is safe")
		return
	}
	if key == "" {
		writeRecoveryLedgerError(w, http.StatusBadRequest, "idempotency_key is required to make replay safe under retries")
		return
	}
	// Refusal before the claim (mirrors refuseUnsafeExplicitReplay at
	// the pin): unsafe work_item_ids without force refuse the whole
	// request and consume no key.
	if !force && len(ids) > 0 && failureClass == "" {
		for _, id := range ids {
			if id == "w-unsafe" {
				writeRecoveryLedgerJSON(w, http.StatusUnprocessableEntity, map[string]any{
					"status": "refused",
					"reason": "unsafe failure class",
					"detail": "nothing was replayed and the idempotency key was not consumed",
				})
				return
			}
		}
	}
	if !force && (failureClass == "projection_bug" || failureClass == "input_invalid") {
		writeRecoveryLedgerJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"status": "refused", "failure_class": failureClass,
			"reason": "unsafe failure class", "detail": "set force=true to replay this class after addressing the cause",
		})
		return
	}

	limitInt := int(limit)
	if limitInt <= 0 {
		limitInt = 100
	}
	var idStrs []string
	for _, id := range ids {
		idStrs = append(idStrs, fmt.Sprintf("%v", id))
	}
	sort.Strings(idStrs)
	parts := append(idStrs, scopeID, stage, failureClass, fmt.Sprint(limitInt), fmt.Sprint(force))
	fingerprint := recoveryFingerprint(parts...)

	effect := map[string]any{
		"status": "replayed", "replayed_count": 2,
		"work_item_ids":   []string{"w1", "w2"},
		"idempotency_key": key, "duplicate": false,
	}
	claimed, outcome, status, detail := c.claim(key, fingerprint, effect)
	if status != http.StatusOK {
		writeRecoveryLedgerError(w, status, detail)
		return
	}
	if !claimed {
		outcome = map[string]any{
			"status": "replayed", "replayed_count": outcome["replayed_count"],
			"work_item_ids":   outcome["work_item_ids"],
			"idempotency_key": key, "duplicate": true,
		}
	}
	writeRecoveryLedgerJSON(w, http.StatusOK, outcome)
}

func (c *recoveryLedgerCore) serveReopen(w http.ResponseWriter, body map[string]any) {
	domain, _ := body["domain"].(string)
	scopeID, _ := body["scope_id"].(string)
	reason, _ := body["reason"].(string)
	key, _ := body["idempotency_key"].(string)
	limit, _ := body["limit"].(float64)

	switch domain {
	case "repo_dependency", "workload_materialization", "submodule_pin":
	default:
		writeRecoveryLedgerJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"status": "refused", "domain": domain,
			"reason": "unknown reopen domain: want one of repo_dependency, workload_materialization, submodule_pin",
			"detail": "other domains fail closed",
		})
		return
	}
	if scopeID == "" {
		writeRecoveryLedgerError(w, http.StatusBadRequest, "scope_id is required")
		return
	}
	if reason == "" {
		writeRecoveryLedgerError(w, http.StatusBadRequest, "reason is required and must explain why the reopen is safe")
		return
	}
	if key == "" {
		writeRecoveryLedgerError(w, http.StatusBadRequest, "idempotency_key is required to make reopen safe under retries")
		return
	}
	// Resolve before the claim (mirrors ResolveReopenTarget at the
	// pin): an unknown scope refuses without consuming the key, so a
	// corrected retry may reuse it.
	if scopeID == "nope" {
		writeRecoveryLedgerError(w, http.StatusNotFound, "no ingestion scope matches scope_id")
		return
	}

	limitInt := int(limit)
	if limitInt <= 0 {
		limitInt = 1000
	}
	fingerprint := recoveryFingerprint(domain, scopeID, fmt.Sprint(limitInt))
	effect := map[string]any{
		"status": "reopened", "domain": domain, "scope_id": scopeID,
		"generation_id": "g7", "reopened_total_count": 2,
		"idempotency_key": key, "duplicate": false,
	}
	claimed, outcome, status, detail := c.claim(key, fingerprint, effect)
	if status != http.StatusOK {
		writeRecoveryLedgerError(w, status, detail)
		return
	}
	if !claimed {
		outcome = map[string]any{
			"status": "reopened", "domain": domain, "scope_id": scopeID,
			"generation_id": "g7", "reopened_total_count": 2,
			"idempotency_key": key, "duplicate": true,
		}
	}
	writeRecoveryLedgerJSON(w, http.StatusOK, outcome)
}

func (c *recoveryLedgerCore) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.hits++

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			c.t.Errorf("stub read body: %v", err)
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			writeRecoveryLedgerError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		switch r.URL.Path {
		case "/api/v0/admin/replay":
			c.serveReplay(w, body)
		case "/api/v0/admin/reopen":
			c.serveReopen(w, body)
		case "/api/v0/admin/recover-generations":
			c.serveRecoverGenerations(w, body)
		case "/api/v0/admin/refinalize":
			c.serveRefinalize(w, body)
		default:
			writeRecoveryLedgerError(w, http.StatusNotFound, "unknown recovery route")
		}
	})
}

func (c *recoveryLedgerCore) seedInProgress(key, fingerprint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys[key] = &recoveryLedgerEntry{fingerprint: fingerprint, completed: false}
}

// snapshot returns the ledger rows, completed effects, and forwarded
// hits under the fake's lock, for the ledger-state assertions below.
func (c *recoveryLedgerCore) snapshot() (map[string]recoveryLedgerEntry, int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rows := make(map[string]recoveryLedgerEntry, len(c.keys))
	for key, entry := range c.keys {
		rows[key] = *entry
	}
	return rows, c.effects, c.hits
}

// postRecovery delivers one recovery POST through dest and decodes the
// JSON body.
func postRecovery(t *testing.T, handler http.Handler, path, payload string) (int, map[string]any) {
	t.Helper()
	req := authedRequest(http.MethodPost, path)
	req.Body = io.NopCloser(strings.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Result().Body)
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("POST %s: decode body %q: %v", path, body, err)
	}
	return rec.Code, decoded
}

// TestRecoveryNative_DuplicateDeliveryCollapsesToOneEffect is the
// center of this family PR: the idempotency-ledger concurrency proof.
// Two identical replay POSTs cross dest (both forwarded — dest neither
// retries nor dedups), the ledger records exactly one effect, the
// second answer replays the stored outcome, and the asserted ledger
// row — not just the HTTP bytes — is the completed claim for that key
// and fingerprint.
func TestRecoveryNative_DuplicateDeliveryCollapsesToOneEffect(t *testing.T) {
	t.Parallel()

	core := newRecoveryLedgerCore(t)
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	payload := `{"scope_id":"s1","reason":"retry after the fix deployed","idempotency_key":"k1"}`
	var firstWorkItems any
	for i, wantDuplicate := range []bool{false, true} {
		code, decoded := postRecovery(t, handler, "/api/v0/admin/replay", payload)
		if code != http.StatusOK {
			t.Fatalf("delivery %d: status = %d, want 200", i+1, code)
		}
		if decoded["status"] != "replayed" {
			t.Errorf("delivery %d: status = %v, want replayed", i+1, decoded["status"])
		}
		if decoded["duplicate"] != wantDuplicate {
			t.Errorf("delivery %d: duplicate = %v, want %v", i+1, decoded["duplicate"], wantDuplicate)
		}
		if i == 0 {
			firstWorkItems = decoded["work_item_ids"]
			continue
		}
		if fmt.Sprintf("%v", decoded["work_item_ids"]) != fmt.Sprintf("%v", firstWorkItems) {
			t.Errorf("duplicate replay = %v, want the stored outcome %v", decoded["work_item_ids"], firstWorkItems)
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
	if row.fingerprint != recoveryFingerprint("s1", "", "", "100", "false") {
		t.Errorf("ledger key k1 fingerprint = %q, want the replay-target fingerprint", row.fingerprint)
	}
}

// TestRecoveryNative_KeyReuseAcrossTargetsIs409 pins the other half of
// the ledger contract: the same key with a different target is a 409
// that records no new effect, and the original target still replays
// its stored outcome afterwards.
func TestRecoveryNative_KeyReuseAcrossTargetsIs409(t *testing.T) {
	t.Parallel()

	core := newRecoveryLedgerCore(t)
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	if code, _ := postRecovery(t, handler, "/api/v0/admin/replay",
		`{"scope_id":"s1","reason":"first target","idempotency_key":"k1"}`); code != http.StatusOK {
		t.Fatalf("first delivery: status = %d, want 200", code)
	}
	if code, decoded := postRecovery(t, handler, "/api/v0/admin/replay",
		`{"scope_id":"s2","reason":"other target","idempotency_key":"k1"}`); code != http.StatusConflict {
		t.Fatalf("reused key: status = %d (%v), want 409", code, decoded)
	}
	if code, decoded := postRecovery(t, handler, "/api/v0/admin/replay",
		`{"scope_id":"s1","reason":"first target","idempotency_key":"k1"}`); code != http.StatusOK || decoded["duplicate"] != true {
		t.Fatalf("original target redelivery: status = %d duplicate = %v, want 200/true", code, decoded["duplicate"])
	}

	_, effects, _ := core.snapshot()
	if effects != 1 {
		t.Errorf("ledger effects = %d, want exactly 1 (the 409 records nothing)", effects)
	}
}

// TestRecoveryNative_InProgressKeyIs409 pins the stranded-claim half
// of the ledger contract: a key whose row is still in_progress answers
// 409, and the row stays in_progress for the operator to inspect.
func TestRecoveryNative_InProgressKeyIs409(t *testing.T) {
	t.Parallel()

	core := newRecoveryLedgerCore(t)
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	core.seedInProgress("k-stranded", recoveryFingerprint("workload_materialization", "s1", "1000"))
	code, decoded := postRecovery(t, handler, "/api/v0/admin/reopen",
		`{"domain":"workload_materialization","scope_id":"s1","reason":"hand repair","idempotency_key":"k-stranded"}`)
	if code != http.StatusConflict {
		t.Fatalf("stranded key: status = %d (%v), want 409", code, decoded)
	}
	rows, effects, _ := core.snapshot()
	if effects != 0 {
		t.Errorf("ledger effects = %d, want 0 (the 409 records nothing)", effects)
	}
	if row := rows["k-stranded"]; row.completed {
		t.Errorf("ledger key k-stranded completed = true, want the row to stay in_progress")
	}
}

// TestRecoveryNative_RefusalBeforeClaimLeavesKeyUnconsumed pins the
// resolve-before-claim half of the protocol on both ledger routes: a
// refused delivery consumes no key, so the corrected retry reuses it.
// The reopen refusal is the unknown-scope 404; the replay refusal is
// the unsafe-class 422; both redeliver the same key to a 200.
func TestRecoveryNative_RefusalBeforeClaimLeavesKeyUnconsumed(t *testing.T) {
	t.Parallel()

	core := newRecoveryLedgerCore(t)
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	if code, _ := postRecovery(t, handler, "/api/v0/admin/reopen",
		`{"domain":"workload_materialization","scope_id":"nope","reason":"hand repair","idempotency_key":"k-reopen"}`); code != http.StatusNotFound {
		t.Fatalf("unknown scope: status = %d, want 404", code)
	}
	if code, decoded := postRecovery(t, handler, "/api/v0/admin/reopen",
		`{"domain":"workload_materialization","scope_id":"s1","reason":"hand repair","idempotency_key":"k-reopen"}`); code != http.StatusOK || decoded["duplicate"] == true {
		t.Fatalf("corrected reopen: status = %d duplicate = %v, want 200/false (the refusal consumed no key)", code, decoded["duplicate"])
	}

	if code, _ := postRecovery(t, handler, "/api/v0/admin/replay",
		`{"work_item_ids":["w-unsafe"],"reason":"retry","idempotency_key":"k-replay"}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("unsafe ids: status = %d, want 422", code)
	}
	if code, decoded := postRecovery(t, handler, "/api/v0/admin/replay",
		`{"work_item_ids":["w-unsafe"],"reason":"cause addressed","idempotency_key":"k-replay","force":true}`); code != http.StatusOK || decoded["duplicate"] == true {
		t.Fatalf("forced replay: status = %d duplicate = %v, want 200/false (the refusal consumed no key)", code, decoded["duplicate"])
	}

	rows, effects, _ := core.snapshot()
	if effects != 2 {
		t.Errorf("ledger effects = %d, want exactly 2 (one per corrected retry)", effects)
	}
	if _, ok := rows["k-reopen"]; !ok {
		t.Errorf("ledger lacks key k-reopen: %v", rows)
	}
	if _, ok := rows["k-replay"]; !ok {
		t.Errorf("ledger lacks key k-replay: %v", rows)
	}
}

// TestRecoveryNative_RecoverGenerationsLedgerProtocol pins the ledger
// half of the recover-generations route: a duplicate delivery replays
// the stored outcome, a key reused across the two modes (scope list vs
// all_scopes) is a 409, and both deliveries forward exactly once.
