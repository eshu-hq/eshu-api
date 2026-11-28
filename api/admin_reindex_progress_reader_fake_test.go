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

// fakeMaintenanceStore is the in-memory apilib MaintenanceStore the
// lane matrix tests run against: it records the scope selection and
// call counts and answers canned watermarks or a canned error. It
// emits no spans by design: span evidence must come from the
// production reader store path (internal/adminread over a fake pool,
// see the P-READ proof), never from a fake MaintenanceStore.
type fakeMaintenanceStore struct {
	fleet    time.Time
	perRepo  map[string]time.Time
	states   map[string]apilibadmin.FullReconcileState
	err      error
	gotIDs   [][]string
	calls    int
	scopeIDs []string
}

func (f *fakeMaintenanceStore) GetReindexState(_ context.Context, _ string) (apilibadmin.ReindexState, error) {
	f.calls++
	if f.err != nil {
		return apilibadmin.ReindexState{}, f.err
	}
	return apilibadmin.ReindexState{RequestedAt: f.fleet}, nil
}

func (f *fakeMaintenanceStore) RepositoryReindexRequestedAt(_ context.Context, scopeIDs []string) (map[string]time.Time, error) {
	f.calls++
	f.gotIDs = append(f.gotIDs, scopeIDs)
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]time.Time{}
	for k, v := range f.perRepo {
		out[k] = v
	}
	return out, nil
}

func (f *fakeMaintenanceStore) FullReconcileState(_ context.Context, scopeID string) (apilibadmin.FullReconcileState, error) {
	f.calls++
	f.scopeIDs = append(f.scopeIDs, scopeID)
	if f.err != nil {
		return apilibadmin.FullReconcileState{}, f.err
	}
	if state, ok := f.states[scopeID]; ok {
		return state, nil
	}
	return apilibadmin.FullReconcileState{}, nil
}

// errFakeProgressExhausted fails a Scan that arrives with no row
// staged or the wrong column count: a shape mismatch in the fake,
// never a backend value.
var errFakeProgressExhausted = errors.New("fake progress pool: no row staged for scan")

// scriptedProgressRows serves one canned record list behind pgx.Rows:
// the P-READ proof drives the real adminread store read path (SQL,
// scan) without a live database.
type scriptedProgressRows struct {
	records [][]any
	pos     int
	closed  bool
}

func (r *scriptedProgressRows) Next() bool { return r.pos < len(r.records) }

func (r *scriptedProgressRows) Scan(dest ...any) error {
	if r.pos >= len(r.records) {
		return errFakeProgressExhausted
	}
	record := r.records[r.pos]
	r.pos++
	if len(dest) != len(record) {
		return errFakeProgressExhausted
	}
	for i, value := range record {
		switch slot := dest[i].(type) {
		case *string:
			*slot = value.(string)
		case *time.Time:
			*slot = value.(time.Time)
		case *sql.NullString:
			*slot = value.(sql.NullString)
		case *sql.NullTime:
			*slot = value.(sql.NullTime)
		default:
			return errFakeProgressExhausted
		}
	}
	return nil
}

func (r *scriptedProgressRows) Err() error { return nil }

func (r *scriptedProgressRows) Close() { r.closed = true }

func (r *scriptedProgressRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *scriptedProgressRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *scriptedProgressRows) Values() ([]any, error) { return nil, nil }

func (r *scriptedProgressRows) RawValues() [][]byte { return nil }

func (r *scriptedProgressRows) Conn() *pgx.Conn { return nil }

// fakeProgressPool binds one canned progress read per query shape
// behind the adminread Pool seam: the fleet instant, the s1
// per-repository watermark, and the s1 scope state with generation
// g7. It emits no spans by design.
type fakeProgressPool struct {
	at     time.Time
	err    error
	closed bool
}

// Query dispatches on the statement text like a real database
// routing three statements: the fleet statement answers the instant,
// the per-repository statement the s1 watermark, and the scope
// statement the stored full plus g7.
func (p *fakeProgressPool) Query(_ context.Context, statement string, _ ...any) (pgx.Rows, error) {
	if p.err != nil {
		return nil, p.err
	}
	switch {
	case strings.Contains(statement, "runtime_ingester_control"):
		return &scriptedProgressRows{records: [][]any{{p.at}}}, nil
	case strings.Contains(statement, "repository_reindex_requests"):
		return &scriptedProgressRows{records: [][]any{{"s1", p.at}}}, nil
	default:
		return &scriptedProgressRows{records: [][]any{{
			sql.NullTime{Time: p.at, Valid: true},
			sql.NullString{String: "g7", Valid: true},
		}}}, nil
	}
}

func (p *fakeProgressPool) Close() { p.closed = true }
