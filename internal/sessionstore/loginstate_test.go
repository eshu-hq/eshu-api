// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// validLoginStateCreate returns a create record that passes every
// validation gate: known provider kind with a positive issued-to-expiry
// window. Tests mutate one field to trip exactly one gate.
func validLoginStateCreate(now time.Time) LoginStateCreate {
	return LoginStateCreate{
		StateHash:        "state-hash",
		ProviderKind:     LoginStateProviderOIDC,
		ProviderConfigID: "provider-config-1",
		TenantID:         "tenant-a",
		WorkspaceID:      "ws-1",
		IssuedAt:         now,
		ExpiresAt:        now.Add(10 * time.Minute),
	}
}

func TestLoginStateCreate_ValidationRejectsBeforeDB(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	cases := map[string]func(*LoginStateCreate){
		"empty state hash":    func(r *LoginStateCreate) { r.StateHash = "" },
		"empty provider kind": func(r *LoginStateCreate) { r.ProviderKind = "" },
		"unknown provider kind": func(r *LoginStateCreate) {
			r.ProviderKind = "pkce-magic"
		},
		"empty provider config": func(r *LoginStateCreate) { r.ProviderConfigID = "" },
		"expired window": func(r *LoginStateCreate) {
			r.ExpiresAt = r.IssuedAt.Add(-time.Minute)
		},
		"zero-width window": func(r *LoginStateCreate) { r.ExpiresAt = r.IssuedAt },
		"zero issued at":    func(r *LoginStateCreate) { r.IssuedAt = time.Time{} },
		"zero expires at":   func(r *LoginStateCreate) { r.ExpiresAt = time.Time{} },
	}
	for name, mutate := range cases {
		primary, _, calls, _ := newFakePair()
		primary.execRes = &fakeResult{n: 1}
		s := NewStore(primary)
		rec := validLoginStateCreate(now)
		mutate(&rec)
		if err := s.CreateLoginState(context.Background(), rec); err == nil {
			t.Errorf("%s must fail validation", name)
		}
		if len(*calls) != 0 {
			t.Errorf("%s must reject before touching the database, got %v", name, *calls)
		}
	}
}

func TestLoginStateCreate_PlainInsertNeverUpsertsOrDeletes(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	primary, _, _, queries := newFakePair()
	primary.execRes = &fakeResult{n: 1}
	s := NewStore(primary)
	rec := validLoginStateCreate(now)
	rec.SecondaryHash = "relay-hash"
	rec.NonceHash = "nonce-hash"
	rec.ReturnToPath = "/dashboard"
	if err := s.CreateLoginState(context.Background(), rec); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(*queries) != 1 {
		t.Fatalf("create must run exactly one statement, got %d", len(*queries))
	}
	upper := strings.ToUpper((*queries)[0])
	if !strings.Contains(upper, "INSERT INTO LOGIN_STATES") {
		t.Fatalf("create must INSERT INTO login_states, got: %s", (*queries)[0])
	}
	// A hash collision fails loudly: the insert must never resurrect a
	// consumed or expired row via ON CONFLICT.
	if strings.Contains(upper, "ON CONFLICT") {
		t.Fatalf("create must be a plain INSERT with no ON CONFLICT upsert: %s", (*queries)[0])
	}
	if strings.Contains(upper, "DELETE") {
		t.Fatalf("create must never DELETE: %s", (*queries)[0])
	}
	if strings.Contains((*queries)[0], "state-hash") {
		t.Fatalf("hash must travel as an argument, never interpolated: %s", (*queries)[0])
	}
}

func TestLoginStateCreate_StoreErrorPropagates(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	primary, _, _, _ := newFakePair()
	boom := errors.New("primary unreachable")
	primary.execErr = boom
	s := NewStore(primary)
	if err := s.CreateLoginState(context.Background(), validLoginStateCreate(now)); !errors.Is(err, boom) {
		t.Fatalf("store error must propagate (caller denies 503), got %v", err)
	}
	if got := s.Stats().StoreErrors.Load(); got != 1 {
		t.Fatalf("store-error counter = %d, want 1", got)
	}
}

func TestLoginStateConsume_HappyPathReturnsStoredFields(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	primary, _, _, queries := newFakePair()
	expires := now.Add(5 * time.Minute)
	primary.row = &fakeRow{values: []any{"provider-config-1", "tenant-a", "ws-1", "nonce-hash", "/dashboard", expires}}
	s := NewStore(primary)
	got, ok, err := s.ConsumeLoginState(context.Background(), LoginStateConsume{
		StateHash:     "state-hash",
		ProviderKind:  LoginStateProviderOIDC,
		SecondaryHash: "",
	}, now)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if !ok {
		t.Fatal("consume of a live row must return ok=true")
	}
	if got.ProviderConfigID != "provider-config-1" || got.TenantID != "tenant-a" ||
		got.WorkspaceID != "ws-1" || got.NonceHash != "nonce-hash" ||
		got.ReturnToPath != "/dashboard" || !got.ExpiresAt.Equal(expires) {
		t.Fatalf("unexpected login state: %+v", got)
	}
	if got := s.Stats().LoginStateConsumed.Load(); got != 1 {
		t.Fatalf("consumed counter = %d, want 1", got)
	}
	q := (*queries)[0]
	upper := strings.ToUpper(q)
	for _, want := range []string{"UPDATE", "LOGIN_STATES", "CONSUMED_AT IS NULL", "RETURNING"} {
		if !strings.Contains(upper, want) {
			t.Fatalf("consume must be a single guarded UPDATE ... RETURNING with %q, got: %s", want, q)
		}
	}
	// Hermetic boundary guard: the strict expires_at > now form (not
	// >=) is what makes expires_at == now dead. A > vs >= mutation
	// must fail this test; the live fixture is the semantic backstop.
	if !strings.Contains(upper, "EXPIRES_AT > ") {
		t.Fatalf("consume must carry the strict expires_at > now predicate, got: %s", q)
	}
	for _, want := range []string{"provider_kind", "secondary_hash"} {
		if !strings.Contains(strings.ToLower(q), want) {
			t.Fatalf("consume must scope the row by %s, got: %s", want, q)
		}
	}
	if strings.Contains(upper, "DELETE") {
		t.Fatalf("consume must stamp consumed_at, never DELETE: %s", q)
	}
	if strings.Contains(upper, "SELECT") && !strings.HasPrefix(strings.TrimSpace(upper), "UPDATE") {
		t.Fatalf("consume must not SELECT-then-UPDATE: %s", q)
	}
	if strings.Contains(q, "state-hash") {
		t.Fatalf("hash must travel as an argument, never interpolated: %s", q)
	}
}

func TestLoginStateConsume_DenyTableIsUniform(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	// Every deny — replayed, expired, at-boundary, unknown, wrong
	// provider, wrong secondary — answers (zero,false,nil) so the caller
	// cannot distinguish them.
	cases := map[string]struct {
		row    Row
		rowErr error
		req    LoginStateConsume
	}{
		"replayed consume": {rowErr: sql.ErrNoRows, req: LoginStateConsume{StateHash: "used", ProviderKind: LoginStateProviderOIDC}},
		"expired":          {rowErr: sql.ErrNoRows, req: LoginStateConsume{StateHash: "old", ProviderKind: LoginStateProviderGitHub}},
		"at-boundary":      {rowErr: sql.ErrNoRows, req: LoginStateConsume{StateHash: "edge", ProviderKind: LoginStateProviderSAML, SecondaryHash: "relay"}},
		"unknown":          {rowErr: sql.ErrNoRows, req: LoginStateConsume{StateHash: "missing", ProviderKind: LoginStateProviderOIDC}},
		"wrong provider":   {rowErr: sql.ErrNoRows, req: LoginStateConsume{StateHash: "h", ProviderKind: LoginStateProviderGitHub}},
		"wrong secondary":  {rowErr: sql.ErrNoRows, req: LoginStateConsume{StateHash: "h", ProviderKind: LoginStateProviderSAML, SecondaryHash: "wrong-relay"}},
	}
	for name, tc := range cases {
		primary, _, _, _ := newFakePair()
		primary.row = tc.row
		primary.rowErr = tc.rowErr
		s := NewStore(primary)
		got, ok, err := s.ConsumeLoginState(context.Background(), tc.req, now)
		if err != nil {
			t.Errorf("%s must deny without error, got %v", name, err)
		}
		if ok {
			t.Errorf("%s must resolve ok=false", name)
		}
		if got != (LoginState{}) {
			t.Errorf("%s must return the zero state, got %+v", name, got)
		}
		if n := s.Stats().LoginStateDenied.Load(); n != 1 {
			t.Errorf("%s must count one uniform denial, got %d", name, n)
		}
	}
}

func TestLoginStateConsume_EmptyHashDeniesWithoutDBCall(t *testing.T) {
	t.Parallel()
	primary, _, calls, _ := newFakePair()
	s := NewStore(primary)
	_, ok, err := s.ConsumeLoginState(context.Background(), LoginStateConsume{ProviderKind: LoginStateProviderOIDC}, time.Now())
	if err != nil {
		t.Fatalf("empty hash must deny without error, got %v", err)
	}
	if ok {
		t.Fatal("empty hash must resolve ok=false")
	}
	if len(*calls) != 0 {
		t.Fatalf("empty hash must not touch the database, got %v", *calls)
	}
	if got := s.Stats().LoginStateDenied.Load(); got != 1 {
		t.Fatalf("denied counter = %d, want 1", got)
	}
}

func TestLoginStateConsume_StoreErrorPropagates(t *testing.T) {
	t.Parallel()
	primary, _, _, _ := newFakePair()
	boom := errors.New("primary unreachable")
	primary.rowErr = boom
	s := NewStore(primary)
	_, _, err := s.ConsumeLoginState(context.Background(), LoginStateConsume{StateHash: "h", ProviderKind: LoginStateProviderOIDC}, time.Now())
	if !errors.Is(err, boom) {
		t.Fatalf("store error must propagate (caller denies 503), got %v", err)
	}
	if got := s.Stats().StoreErrors.Load(); got != 1 {
		t.Fatalf("store-error counter = %d, want 1", got)
	}
}

func TestLoginStateConsume_RunsOnPrimaryOnly(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	primary, replica, calls, _ := newFakePair()
	primary.row = &fakeRow{values: []any{"cfg", "t", "w", "", "", now.Add(time.Minute)}}
	s := NewStore(primary, WithReplica(replica, true))
	if err := s.CreateLoginState(context.Background(), validLoginStateCreate(now)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, _, err := s.ConsumeLoginState(context.Background(), LoginStateConsume{StateHash: "state-hash", ProviderKind: LoginStateProviderOIDC}, now); err != nil {
		t.Fatalf("consume: %v", err)
	}
	for _, c := range *calls {
		if strings.HasPrefix(c, "replica:") {
			t.Fatalf("login-state create/consume is read-your-writes and must stay on the primary, got %v", *calls)
		}
	}
}

// loginStateMigrationSQL loads one login-state migration shipped with
// this package.
func loginStateMigrationSQL(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("migrations", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}

func TestLoginStateMigration_TablesAreIdempotentAndGuarded(t *testing.T) {
	t.Parallel()
	got := loginStateMigrationSQL(t, "003_login_state_tables.sql")
	upper := strings.ToUpper(got)
	if !strings.Contains(upper, "CREATE TABLE IF NOT EXISTS LOGIN_STATES") {
		t.Fatal("003 must create login_states idempotently before 004 grants on it")
	}
	if !strings.Contains(upper, "BEGIN") || !strings.Contains(upper, "COMMIT") {
		t.Fatal("003 must run in a single transaction")
	}
	for _, want := range []string{
		"STATE_HASH", "PROVIDER_KIND", "PROVIDER_CONFIG_ID", "TENANT_ID",
		"WORKSPACE_ID", "SECONDARY_HASH", "NONCE_HASH", "RETURN_TO_PATH",
		"ISSUED_AT", "EXPIRES_AT", "CONSUMED_AT",
	} {
		if !strings.Contains(upper, want) {
			t.Fatalf("003 must declare column %s used by the store queries", want)
		}
	}
	for _, kind := range []string{"'OIDC'", "'GITHUB'", "'SAML'"} {
		if !strings.Contains(upper, kind) {
			t.Fatalf("003 must CHECK provider_kind against %s", kind)
		}
	}
	// Hermetic CHECK guard: pin the comparison itself, not the
	// column mentions — deleting the CHECK line must fail this
	// test. The live twice-applied migration is the backstop.
	matched, err := regexp.MatchString(`EXPIRES_AT\s*>\s*ISSUED_AT`, upper)
	if err != nil || !matched {
		t.Fatal("003 must CHECK expires_at > issued_at so callers cannot persist a dead window")
	}
	if !strings.Contains(upper, "PRIMARY KEY") {
		t.Fatal("003 must key login_states on state_hash so parallel consumes contend on one row")
	}
}

func TestLoginStateMigration_GrantsStayLeastPrivilege(t *testing.T) {
	t.Parallel()
	got := loginStateMigrationSQL(t, "004_login_state_least_privilege.sql")
	upper := strings.ToUpper(got)
	if !strings.Contains(upper, "GRANT SELECT, INSERT, UPDATE ON LOGIN_STATES TO ESHU_API_SESSION_RW") {
		t.Fatal("004 must grant exactly SELECT,INSERT,UPDATE on login_states to the read-write role")
	}
	for _, line := range strings.Split(got, "\n") {
		grant := strings.TrimSpace(strings.ToUpper(line))
		if !strings.HasPrefix(grant, "GRANT ") {
			continue
		}
		for _, forbidden := range []string{"DELETE", "TRUNCATE", "DROP", "ALTER", "CREATE"} {
			if strings.Contains(grant, forbidden) {
				t.Fatalf("runtime roles must hold no DELETE/DDL grants, found %q in %q", forbidden, line)
			}
		}
		// Consume is a write on the primary: the read-only role must
		// never see login_states.
		if strings.Contains(grant, "LOGIN_STATES") && strings.Contains(grant, "ESHU_API_SESSION_RO") {
			t.Fatalf("read-only role must hold no grant on login_states: %q", line)
		}
	}
}
