package api

import "context"

// PlanLimits bir API anahtarinin plan kisitlari (Faz 2'de DB'den yuklenir).
type PlanLimits struct {
	PlanCode         string
	AllowedProviders []string // bos = hepsi
	AllowFresh       bool
	AllowPackages    bool
	DailyQuota       int // 0 = sinirsiz
	RatePerMinute    int // 0 = sinirsiz
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
