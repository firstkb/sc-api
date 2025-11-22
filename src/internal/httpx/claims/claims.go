package claims

import "context"

// ctxKey is a private type of key to avoid collisions in the context.
type ctxKey string

const ctxKeyClaims ctxKey = "httpx.claims"

// With returns a new context with the saved claim value.
// The type of the value is not fixed and is set by the application (struct, map, *Claim and etc.).
func With(ctx context.Context, v any) context.Context {
	return context.WithValue(ctx, ctxKeyClaims, v)
}

// FromContext extracts the saved claim from the context (as any).
// The specific application itself casts the type (type assertion) to its own type.
func FromContext(ctx context.Context) any {
	if ctx == nil {
		return nil
	}
	return ctx.Value(ctxKeyClaims)
}
