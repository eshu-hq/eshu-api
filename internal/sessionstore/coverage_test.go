// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestTokenLifecycle_SuccessPaths(t *testing.T) {
	t.Parallel()
	primary, _, calls, _ := newFakePair()
	primary.execRes = &fakeResult{n: 1}
	primary.rows = &fakeRows{rows: [][]any{{"tok-1", "ci-token", time.Now(), time.Now().Add(time.Hour), false}}}
	s := NewStore(primary)
	ctx := context.Background()
	now := time.Now()
	if err := s.CreateLocalIdentityAPIToken(ctx, APITokenCreate{TokenID: "tok-1", TokenHash: "h", SubjectIDHash: "s", Name: "ci-token"}); err != nil {
		t.Fatalf("token create: %v", err)
	}
	items, err := s.ListAPITokensBySubject(ctx, "s", now, 10, 0)
	if err != nil {
		t.Fatalf("token list: %v", err)
	}
	if len(items) != 1 || items[0].TokenID != "tok-1" || items[0].Revoked {
		t.Fatalf("unexpected token items: %+v", items)
	}
	if err := s.RevokeLocalIdentityAPIToken(ctx, APITokenRevoke{TokenID: "tok-1", SubjectIDHash: "s"}); err != nil {
		t.Fatalf("token revoke: %v", err)
	}
	if len(*calls) != 3 {
		t.Fatalf("expected 3 primary calls, got %v", *calls)
	}
}

func TestMutations_StoreErrorPropagates(t *testing.T) {
	t.Parallel()
	boom := errors.New("primary unreachable")
	cases := map[string]func(s *Store) error{
		"create-session": func(s *Store) error {
			return s.CreateBrowserSession(context.Background(), BrowserSessionCreate{SubjectIDHash: "a-very-long-subject-hash-for-truncation"})
		},
		"revoke-session": func(s *Store) error {
			return s.RevokeBrowserSession(context.Background(), "h", time.Now())
		},
		"create-token": func(s *Store) error {
			return s.CreateLocalIdentityAPIToken(context.Background(), APITokenCreate{})
		},
		"revoke-token": func(s *Store) error {
			return s.RevokeLocalIdentityAPIToken(context.Background(), APITokenRevoke{})
		},
		"list-sessions": func(s *Store) error {
			_, err := s.ListSessionsBySubject(context.Background(), "s", time.Now(), "", 10, 0)
			return err
		},
		"list-tokens": func(s *Store) error {
			_, err := s.ListAPITokensBySubject(context.Background(), "s", time.Now(), 10, 0)
			return err
		},
		"mfa-status": func(s *Store) error {
			_, err := s.GetLocalIdentityMFAStatus(context.Background(), "s", time.Now())
			return err
		},
		"resolve-user": func(s *Store) error {
			_, _, err := s.ResolveLocalIdentityUserID(context.Background(), "s")
			return err
		},
	}
	for name, call := range cases {
		primary, _, _, _ := newFakePair()
		primary.execErr = boom
		primary.queryErr = boom
		primary.rowErr = boom
		s := NewStore(primary)
		if err := call(s); !errors.Is(err, boom) {
			t.Errorf("%s must propagate the store error, got %v", name, err)
		}
	}
}

func TestSwitch_SuccessMovesAllScopesSession(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	now := time.Now().UTC()
	primary.row = &fakeRow{values: []any{"subj", "tenant-b", "ws-9", true, now}}
	s := NewStore(primary)
	got, ok, err := s.SwitchBrowserSessionWorkspace(context.Background(), "h", "tenant-b", "ws-9", now)
	if err != nil || !ok {
		t.Fatalf("switch: ok=%v err=%v", ok, err)
	}
	if got.TenantID != "tenant-b" || got.WorkspaceID != "ws-9" {
		t.Fatalf("unexpected switched context: %+v", got)
	}
}

func TestIdentityReads_SuccessAndUnknown(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	now := time.Now().UTC()
	primary.row = &fakeRow{values: []any{true, now}}
	s := NewStore(primary)
	status, err := s.GetLocalIdentityMFAStatus(context.Background(), "s", now)
	if err != nil || !status.Enrolled {
		t.Fatalf("mfa status: %+v err=%v", status, err)
	}
	primary.row = &fakeRow{values: []any{"user-1"}}
	userID, ok, err := s.ResolveLocalIdentityUserID(context.Background(), "s")
	if err != nil || !ok || userID != "user-1" {
		t.Fatalf("resolve user: %q ok=%v err=%v", userID, ok, err)
	}
	if _, ok, _ := s.ResolveLocalIdentityUserID(context.Background(), ""); ok {
		t.Fatal("empty subject hash must resolve ok=false")
	}
	primary.rowErr = sql.ErrNoRows
	if _, ok, _ := s.ResolveLocalIdentityUserID(context.Background(), "missing"); ok {
		t.Fatal("unknown subject must resolve ok=false")
	}
	if _, err := s.GetLocalIdentityMFAStatus(context.Background(), "missing", now); err != nil {
		t.Fatalf("unknown MFA subject reads as unenrolled without error, got %v", err)
	}
	primary.rowErr = nil
}

func TestCredential_StringRedactsDSN(t *testing.T) {
	t.Setenv("ESHU_TEST_447_RW_DSN", "postgres://eshu-rw:FAKE@db:5432/eshu")
	cred, err := LoadCredentialFromEnv(RoleSessionRW, "ESHU_TEST_447_RW_DSN", "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cred.String(); strings.Contains(got, "FAKE") || !strings.Contains(got, "eshu_api_session_rw") {
		t.Fatalf("String must carry role but never the value: %q", got)
	}
	if cred.Role() != RoleSessionRW || cred.DSN() == "" || cred.Version() == "" {
		t.Fatal("credential accessors must report role, value, and version")
	}
}

func TestRemainingErrorBranches(t *testing.T) {
	t.Parallel()
	boom := errors.New("primary unreachable")
	primary, _, _, _ := newFakePair()
	primary.execErr = boom
	primary.rowErr = boom
	primary.queryErr = boom
	s := NewStore(primary)
	ctx := context.Background()
	if _, _, err := s.SwitchBrowserSessionWorkspace(ctx, "h", "t", "w", time.Now()); !errors.Is(err, boom) {
		t.Errorf("switch must propagate store errors, got %v", err)
	}
	if err := s.RotateLocalIdentityAPIToken(ctx, APITokenRotate{}); !errors.Is(err, boom) {
		t.Errorf("rotate must propagate store errors, got %v", err)
	}
	// Short-hash create error covers the untruncated log-label branch.
	if err := s.CreateBrowserSession(ctx, BrowserSessionCreate{SubjectIDHash: "short"}); !errors.Is(err, boom) {
		t.Errorf("create must propagate store errors, got %v", err)
	}
	// Scan-error branches on both list paths.
	primary.execErr = nil
	primary.rowErr = nil
	primary.queryErr = nil
	primary.rows = &fakeRows{rows: [][]any{{"only-one-column"}}}
	if _, err := s.ListSessionsBySubject(ctx, "s", time.Now(), "", 10, 0); err == nil {
		t.Error("short session row must fail scanning")
	}
	primary.rows = &fakeRows{rows: [][]any{{"only-one-column"}}}
	if _, err := s.ListAPITokensBySubject(ctx, "s", time.Now(), 10, 0); err == nil {
		t.Error("short token row must fail scanning")
	}
}

func TestSQLAdapter_QueryErrorPropagates(t *testing.T) {
	t.Parallel()
	// A closed pool surfaces the query error path in the adapter.
	db := sql.OpenDB(&fakeConnector{conn: &fakeConn{}})
	db.Close()
	adapted := &SQLDB{DB: db}
	if _, err := adapted.QueryContext(context.Background(), "SELECT 1"); err == nil {
		t.Fatal("adapter must propagate query errors from a closed pool")
	}
}

func TestOptions_NilLoggerAndDisabledReplica(t *testing.T) {
	t.Parallel()
	primary, replica, calls, _ := newFakePair()
	primary.rows = &fakeRows{}
	s := NewStore(primary, WithLogger(nil), WithReplica(nil, true), WithReplica(replica, false))
	if _, err := s.ListSessionsBySubject(context.Background(), "s", time.Now(), "", 10, 0); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(*calls) != 1 || (*calls)[0] != "primary:query" {
		t.Fatalf("nil/disabled replica must keep lists on primary, got %v", *calls)
	}
	custom := slog.New(slog.NewJSONHandler(io.Discard, nil))
	s2 := NewStore(primary, WithLogger(custom))
	if s2.log != custom {
		t.Fatal("WithLogger must install the provided logger")
	}
	_ = NewRotating(Credential{role: RoleSessionRO, version: "v-test"}, nil)
}
