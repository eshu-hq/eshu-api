// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package adminread

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"

	"github.com/eshu-hq/eshu-api/internal/storespans"
)

// Decision reader (satellite eshu-hq/eshu-api#325, the third D3
// store-backed promotion). ListDecisions serves the bounded
// projection-decision inspection read and ListEvidence the
// per-decision evidence fan-out, both over the same SELECT-only
// contract as the work-items and dead-letters lanes: bounded
// SELECTs, no writes, no DDL, and no lock or advisory calls. The
// serving pods authenticate as the SELECT-only role (see package
// doc): the reader grant for this lane is SELECT on
// projection_decisions plus SELECT on projection_decision_evidence.
//
// The access clause mirrors the core store field for field: a shared
// key carries no grants (all-scopes), so the reader predicates stay
// the repository/run pair the caller names, and only a future scoped
// resolver passing grants would restrict further. Dest invents no
// grants of its own.

// ListDecisions serves the bounded projection-decision read: the
// same SELECT, predicates, default limit, and ordering the core
// reader applies, returned as apilib rows.
func (s *Store) ListDecisions(ctx context.Context, f apilibadmin.DecisionQueryFilter) ([]apilibadmin.DecisionRow, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("adminread: store is not configured")
	}
	query, args := buildListDecisionsQuery(f)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("adminread: list decisions: %w", err)
	}
	defer rows.Close()
	items, err := scanDecisions(rows)
	if err != nil {
		return nil, err
	}
	// The P-READ span evidence (satellites eshu-hq/eshu-api#324 and
	// #335): one finished postgresql/read span per successful reader
	// query, recorded through the request context. Outside an
	// instrumented chain the emit is a no-op, so the serving path
	// stays safe with no recorder attached.
	storespans.RecordStoreSpan(ctx, storespans.StoreSpan{
		System:    storespans.StoreSystemPostgreSQL,
		Operation: storespans.StoreOpRead,
		Name:      "admin decisions list",
	})
	return items, nil
}

// ListEvidence serves the per-decision evidence fan-out: the same
// bounded SELECT the core reader applies for one decision, returned
// as apilib rows. An unknown decision reads empty (no rows), never
// an error.
func (s *Store) ListEvidence(ctx context.Context, decisionID string) ([]apilibadmin.EvidenceRow, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("adminread: store is not configured")
	}
	query, args := buildListEvidenceQuery(decisionID)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("adminread: list decision evidence: %w", err)
	}
	defer rows.Close()
	items, err := scanEvidence(rows)
	if err != nil {
		return nil, err
	}
	storespans.RecordStoreSpan(ctx, storespans.StoreSpan{
		System:    storespans.StoreSystemPostgreSQL,
		Operation: storespans.StoreOpRead,
		Name:      "admin decision evidence list",
	})
	return items, nil
}

// decisionRows is the minimal row cursor ListDecisions scans: real
// pgx rows satisfy it, and tests drive scanDecisions without a live
// database through it.
type decisionRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// evidenceRows is the minimal row cursor ListEvidence scans: real
// pgx rows satisfy it, and tests drive scanEvidence without a live
// database through it.
type evidenceRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// buildListDecisionsQuery renders the bounded decision reader
// SELECT: the core column set in core order, the repository/run
// pair, the core-exact decision_type gate (empty binds inert), and
// the created_at ASC + decision_id ASC page with a LIMIT bind
// marker. An absent limit defaults to 100, like the apilib handler
// already does for its own fields. The decision type passes through
// verbatim (nil binds the empty gate), exactly like the core
// reader — dest trims nothing here.
func buildListDecisionsQuery(f apilibadmin.DecisionQueryFilter) (string, []any) {
	decisionType := ""
	if f.DecisionType != nil {
		decisionType = *f.DecisionType
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	var builder strings.Builder
	builder.WriteString(`
SELECT
    decision_id,
    decision_type,
    repository_id,
    source_run_id,
    work_item_id,
    subject,
    confidence_score,
    confidence_reason,
    provenance_summary,
    created_at
FROM projection_decisions
WHERE repository_id = $1
  AND source_run_id = $2
  AND ($3 = '' OR decision_type = $3)
ORDER BY created_at ASC, decision_id ASC LIMIT $4`)
	return builder.String(), []any{f.RepositoryID, f.SourceRunID, decisionType, limit}
}

// buildListEvidenceQuery renders the bounded evidence fan-out
// SELECT: the core column set in core order over
// projection_decision_evidence for one decision, with the
// core-exact created_at ASC + evidence_id ASC page.
func buildListEvidenceQuery(decisionID string) (string, []any) {
	var builder strings.Builder
	builder.WriteString(`
SELECT
    evidence_id,
    decision_id,
    fact_id,
    evidence_kind,
    detail,
    created_at
FROM projection_decision_evidence
WHERE decision_id = $1
ORDER BY created_at ASC, evidence_id ASC`)
	return builder.String(), []any{decisionID}
}

// scanDecisions folds queried rows into apilib rows: the
// provenance_summary JSONB unfolds into the row map (empty stays
// nil), mirroring the core fold.
func scanDecisions(rows decisionRows) ([]apilibadmin.DecisionRow, error) {
	var items []apilibadmin.DecisionRow
	for rows.Next() {
		var item apilibadmin.DecisionRow
		var provenance []byte
		if err := rows.Scan(
			&item.DecisionID,
			&item.DecisionType,
			&item.RepositoryID,
			&item.SourceRunID,
			&item.WorkItemID,
			&item.Subject,
			&item.ConfidenceScore,
			&item.ConfidenceReason,
			&provenance,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("adminread: scan decision: %w", err)
		}
		if len(provenance) > 0 {
			if err := json.Unmarshal(provenance, &item.ProvenanceSummary); err != nil {
				return nil, fmt.Errorf("adminread: unmarshal decision provenance: %w", err)
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("adminread: list decisions: %w", err)
	}
	return items, nil
}

// scanEvidence folds queried rows into apilib rows: fact_id stays
// nil when NULL and the detail JSONB unfolds into the row map
// (empty stays nil), mirroring the core fold.
func scanEvidence(rows evidenceRows) ([]apilibadmin.EvidenceRow, error) {
	var items []apilibadmin.EvidenceRow
	for rows.Next() {
		var item apilibadmin.EvidenceRow
		var factID sql.NullString
		var detail []byte
		if err := rows.Scan(
			&item.EvidenceID,
			&item.DecisionID,
			&factID,
			&item.EvidenceKind,
			&detail,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("adminread: scan decision evidence: %w", err)
		}
		if factID.Valid {
			item.FactID = &factID.String
		}
		if len(detail) > 0 {
			if err := json.Unmarshal(detail, &item.Detail); err != nil {
				return nil, fmt.Errorf("adminread: unmarshal evidence detail: %w", err)
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("adminread: list decision evidence: %w", err)
	}
	return items, nil
}
