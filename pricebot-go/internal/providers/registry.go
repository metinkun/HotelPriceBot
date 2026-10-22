// Package providers tum saglayicilari kaydeder ve isimle erisim saglar.
package providers

import (
	"fmt"
	"strings"

	"pricebot/internal/apperr"
	"pricebot/internal/browser"
	"pricebot/internal/config"
	"pricebot/internal/provider"
	"pricebot/internal/providers/enuygun"
	"pricebot/internal/providers/etstur"
	"pricebot/internal/providers/gezinomi"
	"pricebot/internal/providers/jolly"
	"pricebot/internal/providers/setur"
	"pricebot/internal/providers/tatilbudur"
	"pricebot/internal/providers/tatildukkani"
	"pricebot/internal/providers/tatilsepeti"
	"pricebot/internal/providers/touristica"
)

type Registry struct {
	order  []string
	byName map[string]provider.Provider
}

// New tum provider'lari TS surumuyle ayni sirada kaydeder.
func New(cfg *config.Config, b *browser.Manager) *Registry {
	r := &Registry{byName: map[string]provider.Provider{}}
	r.register(etstur.New(cfg.EtsturCookie))
	r.register(tatilsepeti.New(cfg.TatilSepetiCookie, cfg.TatilSepetiVerificationToken))
	r.register(jolly.New())
	r.register(tatilbudur.New())
	r.register(touristica.New(b, cfg.TouristicaTabs, cfg.TouristicaSessionAge))
	r.register(setur.New())
	r.register(gezinomi.New())
	r.register(enuygun.New())
	r.register(tatildukkani.New())
	return r
}

func (r *Registry) register(p provider.Provider) {
	r.order = append(r.order, p.Name())
	r.byName[p.Name()] = p
}

// Get isimle provider doner; bilinmiyorsa 400 hatasi.
func (r *Registry) Get(name string) (provider.Provider, error) {
	p, ok := r.byName[name]
	if !ok {
		return nil, apperr.BadRequestf("Bilinmeyen provider: %s. Desteklenen: %s", name, strings.Join(r.order, ", "))
	}
	return p, nil
}

func (r *Registry) Names() []string { return append([]string{}, r.order...) }

func (r *Registry) All() []provider.Provider {
	out := make([]provider.Provider, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.byName[n])
	}
	return out
}

// PackageProviderOf paket destegi varsa PackageProvider doner.
func PackageProviderOf(p provider.Provider) (provider.PackageProvider, bool) {
	if !p.Capabilities().SupportsPackages {
		return nil, false
	}
	pp, ok := p.(provider.PackageProvider)
	return pp, ok
}

// ResolverOf URL cozumleme destegi varsa URLResolver doner.
func ResolverOf(p provider.Provider) (provider.URLResolver, bool) {
	rr, ok := p.(provider.URLResolver)
	return rr, ok
}

func (r *Registry) String() string { return fmt.Sprint(r.order) }
