package pg

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// txContextKey is an unexported key type to avoid context key collisions.
type txContextKey struct{}

// WithTx returns a new context carrying the provided pgx.Tx.
// If ctx is nil, context.Background() is used. If tx is nil, the original
// context is returned unchanged.
func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if tx == nil {
		return ctx
	}
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext extracts a pgx.Tx previously stored with WithTx.
// The second return value indicates whether a transaction was present.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	if ctx == nil {
		return nil, false
	}
	tx, ok := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx, ok
}
