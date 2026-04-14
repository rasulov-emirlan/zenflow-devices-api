package postgresql

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/logging"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/metrics"
	"github.com/rasulov-emirlan/zenflow-devices-api/pkg/pgxtags"
)

// metricsTracer implements pgx.QueryTracer. It reads op+table tags from ctx
// (set via pgxtags.With by repos) and records counters + latency on each
// query end. Queries without tags fall through with "unknown" labels so we
// still get signal without blowing up cardinality.
type metricsTracer struct{}

// NewMetricsTracer returns a QueryTracer suitable for pgxpool.Config.ConnConfig.Tracer.
func NewMetricsTracer() pgx.QueryTracer { return &metricsTracer{} }

type traceKey struct{}

type traceCtx struct {
	start time.Time
	tags  pgxtags.Tags
}

func (t *metricsTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, traceKey{}, &traceCtx{
		start: time.Now(),
		tags:  pgxtags.FromContext(ctx),
	})
}

func (t *metricsTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	tc, ok := ctx.Value(traceKey{}).(*traceCtx)
	if !ok {
		return
	}
	op := tc.tags.Op
	if op == "" {
		op = "unknown"
	}
	table := tc.tags.Table
	if table == "" {
		table = "unknown"
	}
	outcome := "ok"
	if data.Err != nil {
		outcome = "error"
		logUnexpectedDBErr(ctx, op, table, data.Err)
	}
	metrics.DBQueriesTotal.WithLabelValues(op, table, outcome).Inc()
	metrics.DBQueryDuration.WithLabelValues(op, table).Observe(time.Since(tc.start).Seconds())
}

// logUnexpectedDBErr emits an Error-level log for unexpected errors, including
// pgsqlstate when available. pgx.ErrNoRows is expected and is skipped to avoid
// noise on normal not-found paths.
func logUnexpectedDBErr(ctx context.Context, op, table string, err error) {
	if err == nil {
		return
	}
	// ErrNoRows is a domain-level signal, not a failure.
	if err == pgx.ErrNoRows {
		return
	}
	lg := logging.LoggerFromCtx(ctx)
	attrs := []any{
		slog.String("op", op),
		slog.String("table", table),
		slog.String("err", err.Error()),
	}
	if pgErr, ok := err.(*pgconn.PgError); ok {
		attrs = append(attrs, slog.String("sqlstate", pgErr.Code))
	}
	lg.ErrorContext(ctx, "db query failed", attrs...)
}
