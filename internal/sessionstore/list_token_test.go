// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestList_MetadataOnlyAndReplicaRouted(t *testing.T) {
	t.Parallel()
	primary, replica, calls, queries := newFakePair()
	now := time.Now().UTC()
	replica.rows = &fakeRows{rows: [][]any{{"tenant-a", "ws-1", now, now, now, now, true}}}
	s := NewStore(primary, WithReplica(replica, true))
	items, err := s.ListSessionsBySubject(context.Background(), "subj-hash", now, "sess-hash", 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || !items[0].Current {
		t.Fatalf("unexpected list items: %+v", items)
	}
	if len(*calls) != 1 || (*calls)[0] != "replica:query" {
		t.Fatalf("metadata list must ride the replica when enabled, got %v", *calls)
	}
	// The Current flag is computed server-side by hash comparison, so
	// the hash name legitimately appears inside that single computed
	// expression and past FROM. Strip the parenthesized computation
	// first: a bare hash column anywhere else must fail.
	selectList := strings.ToUpper((*queries)[0])
	if idx := strings.Index(selectList, "FROM"); idx >= 0 {
		selectList = selectList[:idx]
	}
	if start := strings.Index(selectList, "("); start >= 0 {
		if end := strings.Index(selectList[start:], ")"); end >= 0 {
			selectList = selectList[:start] + selectList[start+end+1:]
		}
	}
	for _, forbidden := range []string{"SESSION_HASH", "CSRF_HASH", "TOKEN_HASH"} {
		if strings.Contains(selectList, forbidden) {
			t.Fatalf("metadata list must never select %s: %s", forbidden, (*queries)[0])
		}
	}
}

func TestList_PrimaryByDefault(t *testing.T) {
	t.Parallel()
	primary, replica, calls, _ := newFakePair()
	primary.rows = &fakeRows{}
	s := NewStore(primary, WithReplica(replica, false))
	if _, err := s.ListSessionsBySubject(context.Background(), "subj", time.Now(), "", 50, 0); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(*calls) != 1 || (*calls)[0] != "primary:query" {
		t.Fatalf("lists must default to the primary, got %v", *calls)
	}
}

func TestTokenRotate_ScopesByOwnership(t *testing.T) {
	t.Parallel()
	primary, _, _, queries := newFakePair()
	primary.execRes = &fakeResult{n: 1}
	s := NewStore(primary)
	err := s.RotateLocalIdentityAPIToken(context.Background(), APITokenRotate{
		TokenID: "tok-1", SubjectIDHash: "subj-hash", NewTokenHash: "new-hash",
	})
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	q := strings.ToUpper((*queries)[0])
	if !strings.Contains(q, "UPDATE") {
		t.Fatalf("rotate must be a single UPDATE, got: %s", (*queries)[0])
	}
	if !strings.Contains((*queries)[0], "subject") {
		t.Fatalf("rotate must scope by token ownership: %s", (*queries)[0])
	}
}

func TestIdentityReads_AreSelectOnly(t *testing.T) {
	t.Parallel()
	primary, _, _, queries := newFakePair()
	primary.rows = &fakeRows{}
	primary.row = &fakeRow{values: []any{true, time.Now()}}
	s := NewStore(primary)
	if _, err := s.ListAPITokensBySubject(context.Background(), "subj", time.Now(), 10, 0); err != nil {
		t.Fatalf("token list: %v", err)
	}
	if _, err := s.GetLocalIdentityMFAStatus(context.Background(), "subj", time.Now()); err != nil {
		t.Fatalf("mfa status: %v", err)
	}
	for _, q := range *queries {
		upper := strings.ToUpper(q)
		if !strings.HasPrefix(strings.TrimSpace(upper), "SELECT") {
			t.Fatalf("identity/token reads must be SELECT-only, got: %s", q)
		}
		if strings.Contains(upper, "TOKEN_HASH") && strings.Contains(upper, "SELECT") && strings.Contains(upper, "FROM") {
			t.Fatalf("token list must never return token_hash: %s", q)
		}
	}
}

func TestList_TrimsToLimit(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	now := time.Now().UTC()
	mk := func(tenant string) []any { return []any{tenant, "ws", now, now, now, now, false} }
	primary.rows = &fakeRows{rows: [][]any{mk("tenant-a"), mk("tenant-b")}}
	s := NewStore(primary)
	items, err := s.ListSessionsBySubject(context.Background(), "subj", now, "", 1, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].TenantID != "tenant-a" {
		t.Fatalf("list must return at most limit rows, got %+v", items)
	}
}

func TestTokenCreate_CollisionFailsLoudly(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	primary.execRes = &fakeResult{n: 0}
	s := NewStore(primary)
	err := s.CreateLocalIdentityAPIToken(context.Background(), APITokenCreate{TokenID: "tok-1", TokenHash: "other-hash"})
	if !errors.Is(err, errTokenIDCollision) {
		t.Fatalf("colliding token_id with a different hash must fail loudly, got %v", err)
	}
}

func TestTokenCreate_SameHashRetrySucceeds(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	primary.execRes = &fakeResult{n: 1}
	s := NewStore(primary)
	if err := s.CreateLocalIdentityAPIToken(context.Background(), APITokenCreate{TokenID: "tok-1", TokenHash: "h"}); err != nil {
		t.Fatalf("same-hash retry must succeed, got %v", err)
	}
}

func TestList_NegativeLimitOffsetClampToZero(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	primary.rows = &fakeRows{}
	s := NewStore(primary)
	now := time.Now()
	if items, err := s.ListSessionsBySubject(context.Background(), "s", now, "", -1, -5); err != nil || len(items) != 0 {
		t.Fatalf("negative session limit/offset must clamp to empty: n=%d err=%v", len(items), err)
	}
	if items, err := s.ListAPITokensBySubject(context.Background(), "s", now, -1, -5); err != nil || len(items) != 0 {
		t.Fatalf("negative token limit/offset must clamp to empty: n=%d err=%v", len(items), err)
	}
}

func TestTokenCreate_RowsAffectedErrorPropagates(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	boom := errors.New("rows-affected unreachable")
	primary.execRes = &fakeResult{err: boom}
	s := NewStore(primary)
	if err := s.CreateLocalIdentityAPIToken(context.Background(), APITokenCreate{TokenID: "tok-1"}); !errors.Is(err, boom) {
		t.Fatalf("RowsAffected failure must propagate, got %v", err)
	}
	if got := s.Stats().StoreErrors.Load(); got != 1 {
		t.Fatalf("store-error counter = %d, want 1", got)
	}
}

func TestMFA_NullVerifiedMapsZeroTime(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	primary.row = &fakeRow{values: []any{true}}
	s := NewStore(primary)
	got, err := s.GetLocalIdentityMFAStatus(context.Background(), "s", time.Now())
	if err != nil {
		t.Fatalf("mfa status: %v", err)
	}
	if !got.Enrolled || !got.VerifiedAt.IsZero() {
		t.Fatalf("NULL verified_at must map to zero time, got %+v", got)
	}
}
