package api

import "context"

// Musteri kimligi context anahtari (API key dogrulamasi doldurur).
const clientIDKey ctxKey = "clientId"

func clientIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(clientIDKey).(string); ok {
		return v
	}
	return ""
}
