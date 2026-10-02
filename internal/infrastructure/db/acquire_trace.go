package db

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const (
	// slowAcquireAfter is the pool wait worth reporting: a free connection is handed out in microseconds.
	slowAcquireAfter = 500 * time.Millisecond
	// slowAcquireLogEvery throttles the report so a saturated pool logs a summary, not a line per query.
	slowAcquireLogEvery = 10 * time.Second
)

type acquireStartKey struct{}

// acquireTracer logs slow connection waits separately from query time, with the pool's state.
type acquireTracer struct {
	lastLog atomic.Int64
	slow    atomic.Int64
}

func (*acquireTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return ctx
}

func (*acquireTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (*acquireTracer) TraceAcquireStart(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	return context.WithValue(ctx, acquireStartKey{}, time.Now())
}

func (t *acquireTracer) TraceAcquireEnd(ctx context.Context, pool *pgxpool.Pool, data pgxpool.TraceAcquireEndData) {
	start, ok := ctx.Value(acquireStartKey{}).(time.Time)
	if !ok {
		return
	}
	wait := time.Since(start)
	if wait < slowAcquireAfter {
		return
	}
	slow := t.slow.Add(1)
	now := time.Now().UnixNano()
	last := t.lastLog.Load()
	if now-last < int64(slowAcquireLogEvery) || !t.lastLog.CompareAndSwap(last, now) {
		return
	}
	t.slow.Add(-slow)
	stat := pool.Stat()
	ev := log.Warn()
	if data.Err != nil {
		ev = ev.Err(data.Err)
	}
	ev.Dur("wait", wait).
		Int64("slow_acquires", slow).
		Int32("acquired_conns", stat.AcquiredConns()).
		Int32("total_conns", stat.TotalConns()).
		Int32("max_conns", stat.MaxConns()).
		Int64("canceled_acquires", stat.CanceledAcquireCount()).
		Msg("db: waited for a pooled connection; the pool is saturated")
}
