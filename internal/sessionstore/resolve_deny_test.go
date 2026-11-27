// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestResolve_CSRFMismatchDeniesWithoutWrite(t *testing.T) {
	t.Parallel()
	primary, _, calls, queries := newFakePair()
	now := time.Now().UTC()
	// The guarded UPDATE matches no row (CSRF predicate rejects it);
	// the probe finds a live row with a different CSRF hash.
	primary.rowErr = sql.ErrNoRows
	primary.probe = &fakeRow{values: []any{"other-csrf", false, now.Add(time.Hour), now.Add(24 * time.Hour)}}
	s := NewStore(primary)
	_, ok, err := s.ResolveBrowserSession(context.Background(), "sess-hash", "csrf-hash", true, now)
	if err != nil {
		t.Fatalf("CSRF mismatch must deny without error, got %v", err)
	}
	if ok {
		t.Fatal("CSRF mismatch must resolve ok=false")
	}
	if got := s.Stats().CSRFMismatches.Load(); got != 1 {
		t.Fatalf("csrf counter = %d, want 1", got)
	}
	// Denial must not mutate: exactly one UPDATE (the guarded
	// refresh that matched nothing) plus the read-only probe.
	var updates int
	for _, q := range *queries {
		if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(q)), "UPDATE") {
			updates++
		}
	}
	if updates != 1 {
		t.Fatalf("CSRF deny must issue exactly the one guarded UPDATE, got %d in %v", updates, *calls)
	}
	probe := (*queries)[len(*queries)-1]
	if strings.Contains(strings.ToUpper(probe), "UPDATE") || !strings.Contains(probe, "session-exists-probe") {
		t.Fatalf("deny classification must run the read-only probe, got: %s", probe)
	}
}

func TestResolve_IdleExpiryDeniesStale(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	now := time.Now().UTC()
	primary.rowErr = sql.ErrNoRows
	primary.probe = &fakeRow{values: []any{"csrf-hash", false, now.Add(-time.Minute), now.Add(24 * time.Hour)}}
	s := NewStore(primary)
	_, ok, err := s.ResolveBrowserSession(context.Background(), "sess-hash", "csrf-hash", true, now)
	if err != nil {
		t.Fatalf("idle-expired session must deny without error, got %v", err)
	}
	if ok {
		t.Fatal("idle-expired session must resolve ok=false")
	}
	if got := s.Stats().StaleDenials.Load(); got != 1 {
		t.Fatalf("stale counter = %d, want 1", got)
	}
}

func TestResolve_RevokedSessionDeniesStale(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	now := time.Now().UTC()
	primary.rowErr = sql.ErrNoRows
	primary.probe = &fakeRow{values: []any{"csrf-hash", true, now.Add(time.Hour), now.Add(24 * time.Hour)}}
	s := NewStore(primary)
	_, ok, err := s.ResolveBrowserSession(context.Background(), "sess-hash", "csrf-hash", true, now)
	if err != nil {
		t.Fatalf("revoked session must deny without error, got %v", err)
	}
	if ok {
		t.Fatal("revoked session must resolve ok=false")
	}
	if got := s.Stats().StaleDenials.Load(); got != 1 {
		t.Fatalf("stale counter = %d, want 1", got)
	}
}
func TestClassifyDeny_ProbeErrorFailsClosed(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	boom := errors.New("probe unreachable")
	primary.rowErr = sql.ErrNoRows
	primary.probeErr = boom
	s := NewStore(primary)
	_, ok, err := s.ResolveBrowserSession(context.Background(), "sess-hash", "csrf", true, time.Now())
	if err != nil {
		t.Fatalf("probe failure must deny without error, got %v", err)
	}
	if ok {
		t.Fatal("probe failure must resolve ok=false")
	}
	if got := s.Stats().StoreErrors.Load(); got != 1 {
		t.Fatalf("store-error counter = %d, want 1", got)
	}
	if got := s.Stats().Unresolved.Load(); got != 1 {
		t.Fatalf("unresolved counter = %d, want 1", got)
	}
}

func TestClassifyDeny_LiveProbeWithoutCSRFCountsUnresolved(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	now := time.Now().UTC()
	primary.rowErr = sql.ErrNoRows
	primary.probe = &fakeRow{values: []any{"csrf-hash", false, now.Add(time.Hour), now.Add(24 * time.Hour)}}
	s := NewStore(primary)
	_, ok, err := s.ResolveBrowserSession(context.Background(), "sess-hash", "", false, now)
	if err != nil || ok {
		t.Fatalf("live-probe race must deny without error: ok=%v err=%v", ok, err)
	}
	if got := s.Stats().Unresolved.Load(); got != 1 {
		t.Fatalf("unresolved counter = %d, want 1", got)
	}
}
