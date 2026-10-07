package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/warmbly/warmbly/internal/errx"
)

type relationErrorRows struct {
	pgx.Rows
	err    error
	closed bool
}

func (*relationErrorRows) Next() bool   { return false }
func (r *relationErrorRows) Err() error { return r.err }
func (r *relationErrorRows) Close()     { r.closed = true }

type relationErrorTx struct {
	pgx.Tx
	rows    pgx.Rows
	queries int
}

func (t *relationErrorTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	t.queries++
	return t.rows, nil
}

func TestSyncRelationStopsBeforeWritesOnDeferredRowsError(t *testing.T) {
	rows := &relationErrorRows{err: errors.New("original row-stream failure")}
	tx := &relationErrorTx{rows: rows}
	_, err := SyncRelation(RelationSyncInput{Tx: tx, Ctx: t.Context(), Table: "email_tags", ColMain: "email", ColRelated: "tag", MainID: "22222222-2222-2222-2222-222222222222", NewValues: []string{"33333333-3333-3333-3333-333333333333"}})
	if err == nil || tx.queries != 1 || !rows.closed {
		t.Fatalf("err=%v queries=%d closed=%t", err, tx.queries, rows.closed)
	}
}

func TestSyncRelationRejectsMalformedIDsBeforeQuery(t *testing.T) {
	tx := &relationErrorTx{}
	_, err := SyncRelation(RelationSyncInput{Tx: tx, NewValues: []string{"cro-c6"}})
	if err == nil || err.Code != errx.BadRequest || tx.queries != 0 {
		t.Fatalf("err=%v queries=%d", err, tx.queries)
	}
}
