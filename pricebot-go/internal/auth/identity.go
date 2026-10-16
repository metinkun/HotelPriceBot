// Package auth istek kimligi (musteri / API anahtari / plan) context tasiyicisi.
package auth

import "context"

// Identity dogrulanmis API anahtarinin kimligi. Admin super-anahtar icin IsAdmin=true.
type Identity struct {
	ClientID   int
	ClientName string
	APIKeyID   int
	PlanCode   string
	IsAdmin    bool
}

type ctxKey struct{}

func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func From(ctx context.Context) *Identity {
	if v, ok := ctx.Value(ctxKey{}).(*Identity); ok {
		return v
	}
	return nil
}
