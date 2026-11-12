package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"pricebot/internal/apperr"
	"pricebot/internal/auth"
	"pricebot/internal/xlsx"
)

// ---- API musterisi otel listesi (yalnizca kendisine bagli oteller) ----

type clientHotelRow struct {
	InternalHotelID string   `json:"hotelId"`
	Name            string   `json:"name"`
	Label           *string  `json:"label,omitempty"`
	City            *string  `json:"city,omitempty"`
	Region          *string  `json:"region,omitempty"`
	Country         *string  `json:"country,omitempty"`
	Stars           *int     `json:"stars,omitempty"`
	Chain           *string  `json:"chain,omitempty"`
	IsActive        bool     `json:"isActive"`
	Providers       []string `json:"providers"`        // bu otel icin sorgulanabilir siteler (eslesme var + site acik + paket izinli)
	Missing         []string `json:"missingProviders"` // izinli olup eslesmesi olmayan siteler
	LinkedAt        *string  `json:"linkedAt,omitempty"`
}

// allowedProviderSet plan/musteri izinli + genel olarak acik siteler.
func (s *Server) allowedProviderSet(pl *PlanLimits) map[string]bool {
	names := s.reg.Names()
	if pl != nil && len(pl.AllowedProviders) > 0 {
		names = pl.AllowedProviders
	}
	if s.sites != nil {
		names = s.sites.FilterEnabled(names)
	}
	out := map[string]bool{}
	for _, n := range names {
		out[n] = true
	}
	return out
}

// hotelRowsFor otel kimlik listesi icin satirlari uretir (ad kunyeden, yoksa eslestirmeden).
func (s *Server) hotelRowsFor(ctx context.Context, ids []string, labels map[string]*string, linked map[string]time.Time, allowed map[string]bool) []clientHotelRow {
	metas, _ := s.db.ListHotelMeta(ctx)
	out := make([]clientHotelRow, 0, len(ids))
	for _, id := range ids {
		row := clientHotelRow{InternalHotelID: id, IsActive: true, Providers: []string{}, Missing: []string{}}
		if labels != nil {
			row.Label = labels[id]
		}
		if t, ok := linked[id]; ok {
			ts := t.Format(time.RFC3339)
			row.LinkedAt = &ts
		}
		if m, ok := metas[id]; ok {
			if m.Name != nil {
				row.Name = *m.Name
			}
			row.City, row.Region, row.Country, row.Stars, row.Chain, row.IsActive = m.City, m.Region, m.Country, m.Stars, m.Chain, m.IsActive
		}
		have := map[string]bool{}
		if hv, err := s.hotels.Get(ctx, id); err == nil {
			if row.Name == "" && hv.HotelName != nil {
				row.Name = *hv.HotelName
			}
			for _, p := range hv.Providers {
				if p.IsActive {
					have[p.Provider] = true
				}
			}
		}
		for _, n := range s.reg.Names() {
			if !allowed[n] {
				continue
			}
			if have[n] {
				row.Providers = append(row.Providers, n)
			} else {
				row.Missing = append(row.Missing, n)
			}
		}
		out = append(out, row)
	}
	return out
}

// clientHotelIDs musterinin listesi (kimlik -> etiket, baglanma zamani).
func (s *Server) clientHotelIDs(ctx context.Context, clientID int) ([]string, map[string]*string, map[string]time.Time, error) {
	links, err := s.db.ListClientHotels(ctx, clientID)
	if err != nil {
		return nil, nil, nil, err
	}
	ids := make([]string, 0, len(links))
	labels := map[string]*string{}
	linked := map[string]time.Time{}
	for _, l := range links {
		ids = append(ids, l.InternalHotelID)
		labels[l.InternalHotelID] = l.Label
		linked[l.InternalHotelID] = l.CreatedAt
	}
	return ids, labels, linked, nil
}

// GET /api/v1/hotels?search=  -> musteriye tanimli oteller. Admin super-anahtar / anonim (gelistirme) tum oteller.
func (s *Server) clientHotels(w http.ResponseWriter, r *http.Request) {
	idn := auth.From(r.Context())
	pl := planFrom(r.Context())
	allowed := s.allowedProviderSet(pl)
	var rows []clientHotelRow
	if idn != nil && !idn.IsAdmin && idn.ClientID > 0 {
		ids, labels, linked, err := s.clientHotelIDs(r.Context(), idn.ClientID)
		if err != nil {
			writeError(w, r, err)
			return
		}
		rows = s.hotelRowsFor(r.Context(), ids, labels, linked, allowed)
	} else {
		all, err := s.hotels.List(r.Context(), "", nil)
		if err != nil {
			writeError(w, r, err)
			return
		}
		ids := make([]string, 0, len(all))
		for _, h := range all {
			ids = append(ids, h.InternalHotelID)
		}
		rows = s.hotelRowsFor(r.Context(), ids, nil, nil, allowed)
	}
	if q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search"))); q != "" {
		f := rows[:0]
		for _, row := range rows {
			if strings.Contains(strings.ToLower(row.Name), q) || strings.Contains(strings.ToLower(row.InternalHotelID), q) || (row.City != nil && strings.Contains(strings.ToLower(*row.City), q)) {
				f = append(f, row)
			}
		}
		rows = f
	}
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name) })
	writeJSON(w, 200, map[string]any{"count": len(rows), "hotels": rows})
}

// requireClientHotels API musterisinin yalnizca kendi listesindeki otelleri sorgulamasini saglar (admin/anonim serbest).
func (s *Server) requireClientHotels(r *http.Request, ids ...string) error {
	idn := auth.From(r.Context())
	if idn == nil || idn.IsAdmin || idn.ClientID == 0 {
		return nil
	}
	for _, id := range ids {
		ok, err := s.db.ClientOwnsHotel(r.Context(), idn.ClientID, id)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.NotFoundf("Otel listenizde yok: %s (GET /api/v1/hotels ile tanimli otelleri gorebilirsiniz)", id)
		}
	}
	return nil
}

// ---- Admin: Excel disa aktarma ----

func (s *Server) writeHotelsXLSX(w http.ResponseWriter, filename string, rows []clientHotelRow, withLabel bool) {
	header := []string{"Otel kimliği (hotelId)", "Otel adı"}
	if withLabel {
		header = append(header, "Etiket")
	}
	header = append(header, "Şehir", "Bölge", "Ülke", "Yıldız", "Zincir", "Durum")
	names := s.reg.Names()
	for _, n := range names {
		header = append(header, s.siteLabel(n))
	}
	data := make([][]xlsx.Cell, 0, len(rows))
	for _, h := range rows {
		rec := []xlsx.Cell{h.InternalHotelID, h.Name}
		if withLabel {
			rec = append(rec, strOr(h.Label))
		}
		status := "aktif"
		if !h.IsActive {
			status = "pasif"
		}
		rec = append(rec, strOr(h.City), strOr(h.Region), strOr(h.Country), intOr(h.Stars), strOr(h.Chain), status)
		have := map[string]bool{}
		for _, p := range h.Providers {
			have[p] = true
		}
		for _, n := range names {
			if have[n] {
				rec = append(rec, "var")
			} else {
				rec = append(rec, "")
			}
		}
		data = append(data, rec)
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_ = xlsx.Write(w, "Oteller", header, data)
}

func (s *Server) siteLabel(n string) string {
	if s.sites != nil {
		return s.sites.Label(n, n)
	}
	return n
}

func strOr(p *string) xlsx.Cell {
	if p == nil {
		return nil
	}
	return *p
}
func intOr(p *int) xlsx.Cell {
	if p == nil {
		return nil
	}
	return *p
}

// GET /api/admin/clients/{id}/hotels/export -> xlsx (musterinin listesi; siteler plan/musteri izinleriyle)
func (s *Server) adminExportClientHotels(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	c, p, err := s.clientWithPlan(r.Context(), id)
	if err != nil {
		notFoundOr(w, r, err, "Musteri bulunamadi")
		return
	}
	ids, labels, linked, err := s.clientHotelIDs(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	rows := s.hotelRowsFor(r.Context(), ids, labels, linked, s.allowedProviderSet(limitsOf(c, p)))
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name) })
	s.writeHotelsXLSX(w, fmt.Sprintf("oteller-musteri-%d-%s.xlsx", id, time.Now().Format("20060102")), rows, true)
}

// GET /api/admin/hotels/export -> xlsx (sistemdeki tum oteller)
func (s *Server) adminExportHotels(w http.ResponseWriter, r *http.Request) {
	all, err := s.hotels.List(r.Context(), r.URL.Query().Get("search"), nil)
	if err != nil {
		writeError(w, r, err)
		return
	}
	ids := make([]string, 0, len(all))
	for _, h := range all {
		ids = append(ids, h.InternalHotelID)
	}
	allowed := map[string]bool{}
	for _, n := range s.reg.Names() {
		allowed[n] = true
	}
	rows := s.hotelRowsFor(r.Context(), ids, nil, nil, allowed)
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name) })
	s.writeHotelsXLSX(w, "oteller-"+time.Now().Format("20060102")+".xlsx", rows, false)
}
