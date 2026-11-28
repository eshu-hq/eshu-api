// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package samlkeys

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
)

// Snapshot is one immutable per-provider signing set: the Current
// key G7 signs new AuthnRequests with, plus the Previous key kept
// published through the rotation overlap. G7 captures one Snapshot
// per AuthnRequest with Get and signs outside any lock, so in-flight
// signing finishes on its captured snapshot after a Reload swaps the
// live one — rotation never blocks or breaks signing.
//
// The zero value is invalid: Get reports absence instead of
// returning it.
type Snapshot struct {
	current     Key
	previous    Key
	hasPrevious bool
}

// Signer returns the CURRENT key's signer for one AuthnRequest. Like
// Key.Signer, the result must never reach logs, labels, or storage.
func (s Snapshot) Signer() crypto.Signer { return s.current.Signer() }

// Certificate returns the CURRENT key's leaf certificate.
func (s Snapshot) Certificate() *x509.Certificate { return s.current.Certificate() }

// MetadataCertificates returns the certificates G7 publishes in SP
// metadata: the current certificate plus the previous one while an
// overlap is open, so the IdP trusts both during rotation. If
// assertions are ever encrypted, the previous SIGNER stays available
// on this snapshot for decryption through the same overlap.
func (s Snapshot) MetadataCertificates() []*x509.Certificate {
	if s.hasPrevious {
		return []*x509.Certificate{s.current.Certificate(), s.previous.Certificate()}
	}
	return []*x509.Certificate{s.current.Certificate()}
}

// PreviousSigner returns the PREVIOUS key's signer, for decrypting
// deliveries made to the previous certificate during the overlap. It
// reports false once DropPrevious ends the overlap. Callers must
// never log, label, or persist the result.
func (s Snapshot) PreviousSigner() (crypto.Signer, bool) {
	if !s.hasPrevious {
		return nil, false
	}
	return s.previous.Signer(), true
}

// Version reports the current key's audit version.
func (s Snapshot) Version() string { return s.current.Version() }

// Fingerprint reports the current key's certificate fingerprint.
func (s Snapshot) Fingerprint() string { return s.current.Fingerprint() }

// HasPrevious reports whether a rotation overlap is open.
func (s Snapshot) HasPrevious() bool { return s.hasPrevious }

// PreviousVersion reports the previous key's audit version, or empty
// when no overlap is open.
func (s Snapshot) PreviousVersion() string {
	if !s.hasPrevious {
		return ""
	}
	return s.previous.Version()
}

// PreviousFingerprint reports the previous key's certificate
// fingerprint, or empty when no overlap is open.
func (s Snapshot) PreviousFingerprint() string {
	if !s.hasPrevious {
		return ""
	}
	return s.previous.Fingerprint()
}

// String redacts the material like Key.String: metadata only.
func (s Snapshot) String() string {
	if s.hasPrevious {
		return fmt.Sprintf("Snapshot{current:%s previous:%s}", s.current.String(), s.previous.String())
	}
	return fmt.Sprintf("Snapshot{current:%s}", s.current.String())
}

// GoString redacts the material like String.
func (s Snapshot) GoString() string { return s.String() }

// Store holds the live per-provider snapshots behind one atomic
// pointer to an immutable map, so rotation swaps the whole provider
// table without dropping in-flight readers: new AuthnRequests pick
// up the replacement while captured snapshots drain. Replicas hold
// no durable state — the table is memory only, rebuilt from secret
// mounts at startup — and every load, rotation, rejection, and
// overlap end emits a structured audit record carrying actor,
// provider, version, fingerprint, and expiry, never key material.
type Store struct {
	current atomic.Pointer[map[string]Snapshot]
	log     *slog.Logger
}

// NewStore publishes an empty provider table. Keys arrive through
// Reload, so a deploy without SAML secrets starts exactly as before
// and G7 wiring decides when custody goes live.
func NewStore(log *slog.Logger) *Store {
	if log == nil {
		log = slog.Default()
	}
	s := &Store{log: log}
	s.current.Store(&map[string]Snapshot{})
	return s
}

// Get returns the live snapshot for one provider, captured for a
// single AuthnRequest. The second result reports absence for unknown
// or empty provider ids instead of failing closed: signing callers
// judge the miss, custody never invents a key.
func (s *Store) Get(provider string) (Snapshot, bool) {
	if strings.TrimSpace(provider) == "" {
		return Snapshot{}, false
	}
	table := s.current.Load()
	if table == nil {
		return Snapshot{}, false
	}
	snap, ok := (*table)[provider]
	return snap, ok
}

// Reload validates the replacement key fully BEFORE publishing: on
// success the live current demotes to previous (opening the overlap)
// and the table swaps atomically; on failure the old table stays
// live untouched and the rejection is audited. The caller passes the
// human or controller identity performing the rotation.
func (s *Store) Reload(actor, provider string, next Key) error {
	// Canonicalize like the loader: a padded id addresses the same
	// entry Get finds, never a ghost.
	provider = strings.TrimSpace(provider)
	if provider == "" {
		s.audit("saml-sp-key-rejected", actor, provider, "", "", time.Time{})
		return fmt.Errorf("samlkeys: refusing to reload with an empty provider id")
	}
	if s.current.Load() == nil {
		s.audit("saml-sp-key-rejected", actor, provider, next.Version(), next.Fingerprint(), time.Time{})
		return fmt.Errorf("samlkeys: refusing to reload provider %q on an uninitialized store", provider)
	}
	if next.Provider() != provider {
		s.audit("saml-sp-key-rejected", actor, provider, next.Version(), next.Fingerprint(), time.Time{})
		return fmt.Errorf("samlkeys: refusing to reload provider %q with a key for provider %q", provider, next.Provider())
	}
	if next.Signer() == nil || next.Certificate() == nil {
		s.audit("saml-sp-key-rejected", actor, provider, next.Version(), next.Fingerprint(), time.Time{})
		return fmt.Errorf("samlkeys: refusing to reload provider %q with an invalid key", provider)
	}
	// Re-check validity at publish time: the clock may have
	// advanced past expiry since the key was loaded, and the
	// fully-validated-before-publish promise is kept here, not at
	// load.
	now := time.Now()
	if cert := next.Certificate(); now.After(cert.NotAfter) || now.Before(cert.NotBefore) {
		s.audit("saml-sp-key-rejected", actor, provider, next.Version(), next.Fingerprint(), next.NotAfter())
		return fmt.Errorf("samlkeys: refusing to reload provider %q with a key outside its validity window", provider)
	}
	old := s.snapshotFor(provider)
	next2 := Snapshot{current: next}
	event := "saml-sp-key-load"
	if old != nil {
		if old.hasPrevious {
			// A second rotation before DropPrevious evicts the
			// overlapped key: audit the eviction with the
			// evicted identity so no lifecycle event goes silent.
			s.audit("saml-sp-key-overlap-evict", actor, provider, old.previous.Version(), old.previous.Fingerprint(), old.previous.NotAfter())
		}
		next2.previous = old.current
		next2.hasPrevious = true
		event = "saml-sp-key-rotation"
	}
	s.swap(provider, next2)
	s.audit(event, actor, provider, next.Version(), next.Fingerprint(), next.NotAfter())
	return nil
}

// ReloadFromFiles loads the replacement from secret-mounted files
// first and reloads only a fully validated key: a bad file leaves
// the live snapshot untouched and audits the rejection. Paths name
// secret mounts, never material; version is the optional audit
// label.
func (s *Store) ReloadFromFiles(actor, provider, keyPath, certPath, version string) error {
	next, err := LoadKeyFromFiles(provider, keyPath, certPath, version)
	if err != nil {
		s.audit("saml-sp-key-rejected", actor, provider, "", "", time.Time{})
		return err
	}
	return s.Reload(actor, provider, next)
}

// DropPrevious ends the rotation overlap for one provider once
// in-flight requests signed before the rotation have expired: the
// previous key leaves the table and metadata shrinks to the current
// certificate. It reports false for unknown or empty provider ids
// and for providers with no overlap open.
func (s *Store) DropPrevious(actor, provider string) bool {
	old := s.snapshotFor(provider)
	if old == nil || !old.hasPrevious {
		return false
	}
	next := Snapshot{current: old.current}
	s.swap(provider, next)
	s.audit("saml-sp-key-overlap-end", actor, provider, next.Version(), next.Fingerprint(), next.current.NotAfter())
	return true
}

// snapshotFor reads one provider's live snapshot without locking:
// the atomic table pointer makes the read race-free.
func (s *Store) snapshotFor(provider string) *Snapshot {
	table := s.current.Load()
	if table == nil {
		return nil
	}
	snap, ok := (*table)[provider]
	if !ok {
		return nil
	}
	dup := snap
	return &dup
}

// swap publishes one provider's snapshot via copy-on-write: the old
// map is never mutated, so readers holding it keep a consistent
// table while the new map goes live atomically.
func (s *Store) swap(provider string, next Snapshot) {
	old := s.current.Load()
	fresh := make(map[string]Snapshot, len(*old)+1)
	for id, snap := range *old {
		fresh[id] = snap
	}
	fresh[provider] = next
	s.current.Store(&fresh)
}

// audit records one key lifecycle event without material: event,
// actor, provider, version, fingerprint, expiry, and timestamp only,
// mirroring the sessionstore Rotating audit shape.
func (s *Store) audit(event, actor, provider, version, fingerprint string, notAfter time.Time) {
	log := s.log
	if log == nil {
		// A zero-value Store still audits refusals instead of
		// panicking; callers should still prefer NewStore.
		log = slog.Default()
	}
	log.Info("saml sp key "+event,
		"event", event,
		"actor", actor,
		"provider", provider,
		"version", version,
		"fingerprint", fingerprint,
		"notAfter", notAfter.UTC().Format(time.RFC3339),
		"at", time.Now().UTC().Format(time.RFC3339),
	)
}
