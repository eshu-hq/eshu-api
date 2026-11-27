// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// CredentialRole names the PostgreSQL role a credential authenticates
// as. Only the two least-privilege session roles exist: the read-write
// role rides the primary, the read-only role rides the replica for
// metadata-only lists. There is deliberately no superuser, owner, or
// shared 'dsn' role.
type CredentialRole string

const (
	// RoleSessionRW authenticates as eshu_api_session_rw against the
	// primary: SELECT/INSERT/UPDATE on the session and token tables,
	// SELECT on identity profile columns. No DELETE, no DDL.
	RoleSessionRW CredentialRole = "eshu_api_session_rw"
	// RoleSessionRO authenticates as eshu_api_session_ro against the
	// read replica: SELECT on metadata-only list surfaces. Never used
	// for ResolveBrowserSession, which is a write.
	RoleSessionRO CredentialRole = "eshu_api_session_ro"
)

// Credential is one loaded database credential: its role plus an
// audit version. The DSN value never leaves this struct except into
// the pool constructor; String and GoString redact it so a stray
// %v in logs cannot leak the secret.
type Credential struct {
	role    CredentialRole
	version string
	dsn     string
}

// Role reports the PostgreSQL role this credential authenticates as.
func (c Credential) Role() CredentialRole { return c.role }

// Version reports the secret version this credential was loaded from
// (explicit version env, else the load timestamp).
func (c Credential) Version() string { return c.version }

// DSN reveals the credential value for the pool constructor only.
// Callers must never log, label, or persist the result.
func (c Credential) DSN() string { return c.dsn }

// String redacts the secret: only role and version render.
func (c Credential) String() string {
	return fmt.Sprintf("Credential{role:%s version:%s}", c.role, c.version)
}

// GoString redacts the secret like String: without it, %#v would
// dump the live DSN into any debug-format log line.
func (c Credential) GoString() string { return c.String() }

// LoadCredentialFromEnv reads one credential from the environment:
// dsnEnv names the secret-mapped DSN variable (mounted from a
// secretKeyRef, never a flag or a default), and versionEnv
// optionally names a variable carrying the secret version for
// audit. Empty input fails closed; unknown roles fail closed.
func LoadCredentialFromEnv(role CredentialRole, dsnEnv, versionEnv string) (Credential, error) {
	if role != RoleSessionRW && role != RoleSessionRO {
		return Credential{}, fmt.Errorf("sessionstore: unknown credential role %q", role)
	}
	dsn := strings.TrimSpace(os.Getenv(dsnEnv))
	if dsn == "" {
		return Credential{}, fmt.Errorf("sessionstore: credential %s (%s) unset, refusing to start", role, dsnEnv)
	}
	version := strings.TrimSpace(os.Getenv(versionEnv))
	if versionEnv == "" || version == "" {
		version = time.Now().UTC().Format(time.RFC3339)
	}
	return Credential{role: role, version: version, dsn: dsn}, nil
}

// Rotating holds the live credential behind an atomic pointer so
// rotation swaps it without dropping in-flight readers: new
// connections pick up the replacement while existing ones drain
// under the pool's max lifetime. Every load and reload emits a
// structured audit record carrying actor, role, timestamp, and
// version — never the credential value.
type Rotating struct {
	current atomic.Pointer[Credential]
	log     *slog.Logger
}

// NewRotating publishes the first credential with an audit record.
func NewRotating(first Credential, log *slog.Logger) *Rotating {
	if log == nil {
		log = slog.Default()
	}
	r := &Rotating{log: log}
	r.current.Store(&first)
	r.audit("credential-load", "startup", first)
	return r
}

// Get returns the live credential for dialing new connections.
func (r *Rotating) Get() Credential { return *r.current.Load() }

// Reload publishes the replacement credential with an audit record.
// The caller passes the human or controller identity performing the
// rotation.
func (r *Rotating) Reload(actor string, next Credential) {
	r.current.Store(&next)
	r.audit("credential-rotation", actor, next)
}

// audit records one credential lifecycle event without the value.
func (r *Rotating) audit(event, actor string, cred Credential) {
	r.log.Info("session credential "+event,
		"event", event,
		"actor", actor,
		"role", string(cred.role),
		"version", cred.version,
		"at", time.Now().UTC().Format(time.RFC3339),
	)
}
