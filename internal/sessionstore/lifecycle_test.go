// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestRevoke_IsIdempotentOnZeroRows(t *testing.T) {
	t.Parallel()
	primary, _, _, queries := newFakePair()
	primary.execRes = &fakeResult{n: 0}
	s := NewStore(primary)
	if err := s.RevokeBrowserSession(context.Background(), "sess-hash", time.Now()); err != nil {
		t.Fatalf("revoke of an already-gone session must succeed, got %v", err)
	}
	q := (*queries)[0]
	if !strings.Contains(strings.ToUpper(q), "UPDATE") {
		t.Fatalf("revoke must UPDATE revoked_at, never DELETE: %s", q)
	}
	if strings.Contains(strings.ToUpper(q), "DELETE") {
		t.Fatalf("revoke must never DELETE: %s", q)
	}
}

func TestSwitch_NonAllScopesRowDenies(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	primary.rowErr = sql.ErrNoRows
	s := NewStore(primary)
	_, ok, err := s.SwitchBrowserSessionWorkspace(context.Background(), "scoped-hash", "t", "w", time.Now())
	if err != nil {
		t.Fatalf("scoped switch must deny without error, got %v", err)
	}
	if ok {
		t.Fatal("scoped session switch must resolve ok=false")
	}
}

func TestCreate_NeverClearsRevocation(t *testing.T) {
	t.Parallel()
	primary, _, _, queries := newFakePair()
	primary.execRes = &fakeResult{n: 1}
	s := NewStore(primary)
	rec := BrowserSessionCreate{SessionHash: "h", IdleExpiresAt: time.Now(), AbsoluteExpiresAt: time.Now().Add(time.Hour)}
	if err := s.CreateBrowserSession(context.Background(), rec); err != nil {
		t.Fatalf("create: %v", err)
	}
	if upper := strings.ToUpper((*queries)[0]); strings.Contains(upper, "REVOKED_AT") {
		t.Fatalf("create upsert must never touch revoked_at so revocations stick: %s", (*queries)[0])
	}
}

func TestSwitch_EmptyHashUnresolvedWithoutDBCall(t *testing.T) {
	t.Parallel()
	primary, _, calls, _ := newFakePair()
	s := NewStore(primary)
	_, ok, err := s.SwitchBrowserSessionWorkspace(context.Background(), "", "t", "w", time.Now())
	if err != nil {
		t.Fatalf("empty hash must deny without error, got %v", err)
	}
	if ok {
		t.Fatal("empty hash must resolve ok=false")
	}
	if len(*calls) != 0 {
		t.Fatalf("empty hash must not touch the database, got %v", *calls)
	}
	if got := s.Stats().Unresolved.Load(); got != 1 {
		t.Fatalf("unresolved counter = %d, want 1", got)
	}
}
