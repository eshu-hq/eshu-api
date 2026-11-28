// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// errFakeDecisionsExhausted fails a Scan that arrives with no row
// staged or the wrong column count: a shape mismatch in the fake,
// never a backend value.
var errFakeDecisionsExhausted = errors.New("fake decisions pool: no row staged for scan")

// fakeDecisionRows serves one canned decision reader row in SELECT
// column order behind pgx.Rows: the P-READ proof drives the real
// adminread store read path (SQL, scan) without a live database.
type fakeDecisionRows struct {
	staged bool
	closed bool
	at     time.Time
}

func (r *fakeDecisionRows) Next() bool { return r.staged }

func (r *fakeDecisionRows) Scan(dest ...any) error {
	if !r.staged || len(dest) != 10 {
		return errFakeDecisionsExhausted
	}
	r.staged = false
	values := []any{
		"d1", "merge", "r1", "s1", "w1", "subject-a", 0.9, "reason-a",
		[]byte(`{"planner":"p"}`),
		r.at,
	}
	for i, value := range values {
		switch slot := dest[i].(type) {
		case *string:
			*slot = value.(string)
		case *float64:
			*slot = value.(float64)
		case *time.Time:
			*slot = value.(time.Time)
		case *[]byte:
			*slot = value.([]byte)
		default:
			return errFakeDecisionsExhausted
		}
	}
	return nil
}

func (r *fakeDecisionRows) Err() error { return nil }

func (r *fakeDecisionRows) Close() { r.closed = true }

// The pgx.Rows remainder: the proof drives Next/Scan/Err/Close only,
// so the rest stay inert.
func (r *fakeDecisionRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *fakeDecisionRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *fakeDecisionRows) Values() ([]any, error) { return nil, nil }

func (r *fakeDecisionRows) RawValues() [][]byte { return nil }

func (r *fakeDecisionRows) Conn() *pgx.Conn { return nil }

// fakeEvidenceRows serves one canned evidence reader row in SELECT
// column order behind pgx.Rows: the P-READ proof drives the real
// evidence fan-out path without a live database.
type fakeEvidenceRows struct {
	staged bool
	closed bool
	at     time.Time
}

func (r *fakeEvidenceRows) Next() bool { return r.staged }

func (r *fakeEvidenceRows) Scan(dest ...any) error {
	if !r.staged || len(dest) != 6 {
		return errFakeDecisionsExhausted
	}
	r.staged = false
	fact := "f1"
	values := []any{
		"e1", "d1",
		sql.NullString{String: fact, Valid: true},
		"file",
		[]byte(`{"p":"x"}`),
		r.at,
	}
	for i, value := range values {
		switch slot := dest[i].(type) {
		case *string:
			*slot = value.(string)
		case *sql.NullString:
			*slot = value.(sql.NullString)
		case *time.Time:
			*slot = value.(time.Time)
		case *[]byte:
			*slot = value.([]byte)
		default:
			return errFakeDecisionsExhausted
		}
	}
	return nil
}

func (r *fakeEvidenceRows) Err() error { return nil }

func (r *fakeEvidenceRows) Close() { r.closed = true }

// The pgx.Rows remainder: the proof drives Next/Scan/Err/Close only,
// so the rest stay inert.
func (r *fakeEvidenceRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *fakeEvidenceRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *fakeEvidenceRows) Values() ([]any, error) { return nil, nil }

func (r *fakeEvidenceRows) RawValues() [][]byte { return nil }

func (r *fakeEvidenceRows) Conn() *pgx.Conn { return nil }

// fakeDecisionsPool binds one canned decision row plus one canned
// evidence row behind the adminread Pool seam: the P-READ proof runs
// the real production store (list plus fan-out) over this pool. The
// SQL text selects the staged rows, so the fan-out leg reads the
// evidence shape, not the decision shape.
type fakeDecisionsPool struct {
	decisions *fakeDecisionRows
	evidence  *fakeEvidenceRows
	err       error
	closed    bool
}

func (p *fakeDecisionsPool) Query(_ context.Context, query string, _ ...any) (pgx.Rows, error) {
	if p.err != nil {
		return nil, p.err
	}
	if strings.Contains(query, "projection_decision_evidence") {
		return p.evidence, nil
	}
	return p.decisions, nil
}

func (p *fakeDecisionsPool) Close() { p.closed = true }

// fakeDecisionStore is the in-memory apilib DecisionStore the lane
// matrix tests run against: it records the filter and call counts
// and answers canned rows or canned errors. It emits no spans by
// design: span evidence must come from the production reader store
// path (internal/adminread over a fake pool, see the P-READ proof
// below), never from a fake DecisionStore.
type fakeDecisionStore struct {
	decisions []apilibadmin.DecisionRow
	evidence  map[string][]apilibadmin.EvidenceRow
	err       error
	evErr     error
	got       apilibadmin.DecisionQueryFilter
	gotEv     []string
	calls     int
	evCalls   int
}

func (f *fakeDecisionStore) ListDecisions(_ context.Context, filter apilibadmin.DecisionQueryFilter) ([]apilibadmin.DecisionRow, error) {
	f.got = filter
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.decisions, nil
}

func (f *fakeDecisionStore) ListEvidence(_ context.Context, decisionID string) ([]apilibadmin.EvidenceRow, error) {
	f.gotEv = append(f.gotEv, decisionID)
	f.evCalls++
	if f.evErr != nil {
		return nil, f.evErr
	}
	return f.evidence[decisionID], nil
}
