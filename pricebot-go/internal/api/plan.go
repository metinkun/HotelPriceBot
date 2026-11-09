package api

import (
	"context"

	"pricebot/internal/db"
)

// PlanLimits bir musterinin gecerli kisitlari: paket ∩ musteri ozellestirmesi (db.Limits) + izinli siteler.
type PlanLimits struct {
	PlanCode         string
	AllowedProviders []string // bos = hepsi
	Kind             string   // acente | otel
	db.Limits
}

// limitsOf musteri + paketten gecerli limitleri uretir (musteri limits alani varsa o kazanir).
func limitsOf(c *db.Client, p *db.Plan) *PlanLimits {
	pl := &PlanLimits{Limits: db.EffectiveLimits(c, p)}
	if p != nil {
		pl.PlanCode = p.Code
		pl.AllowedProviders = p.AllowedProviders
	}
	if c != nil {
		pl.Kind = c.Kind
	}
	return pl
}

const planKey ctxKey = "plan"

func planFrom(ctx context.Context) *PlanLimits {
	if v, ok := ctx.Value(planKey).(*PlanLimits); ok {
		return v
	}
	return nil
}

// FilterProviders istenen provider listesini plana gore daraltir. Istek bos ise izinlilerin tamami.
func (p *PlanLimits) FilterProviders(requested, all []string) []string {
	if p == nil || len(p.AllowedProviders) == 0 {
		return requested
	}
	allowed := map[string]bool{}
	for _, a := range p.AllowedProviders {
		allowed[a] = true
	}
	src := requested
	if len(src) == 0 {
		src = all
	}
	out := []string{}
	for _, n := range src {
		if allowed[n] {
			out = append(out, n)
		}
	}
	return out
}
