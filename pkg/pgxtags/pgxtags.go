// Package pgxtags lets repository code label a query with a (op, table) tuple
// that a pgx.QueryTracer can then read at Query{Start,End} time. Repos stay
// free of metric imports; the tracer stays free of SQL parsing.
package pgxtags

import "context"

type ctxKey struct{}

// Tags carries the operational labels a tracer will emit.
type Tags struct {
	Op    string // e.g. "select", "insert", "update", "delete"
	Table string // e.g. "device_profiles"
}

// With returns a new context carrying op+table tags for the next query.
func With(ctx context.Context, op, table string) context.Context {
	return context.WithValue(ctx, ctxKey{}, Tags{Op: op, Table: table})
}

// FromContext returns the tags previously set with With, or zero-value tags.
func FromContext(ctx context.Context) Tags {
	if ctx == nil {
		return Tags{}
	}
	if t, ok := ctx.Value(ctxKey{}).(Tags); ok {
		return t
	}
	return Tags{}
}
