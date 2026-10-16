package api

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"pricebot/internal/apperr"
	"pricebot/internal/types"
)

var reIATA = regexp.MustCompile(`^[A-Za-z]{3}$`)

const (
	maxAdults   = 8
	maxChildren = 6
	maxNights   = 30
	maxAhead    = 400 // gun
)

func parseDay(s, field string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return t, apperr.BadRequestf("%s gecersiz; yyyy-MM-dd bekleniyor (ornek 2026-07-07)", field)
	}
	return t, nil
}

// validateStay tarih ve kisi bilgisini dogrular; normalize edilmis tarihleri doner.
func validateStay(checkIn, checkOut string, adults, children int, childAges []int) (string, string, error) {
	ci, err := parseDay(checkIn, "checkIn")
	if err != nil {
		return "", "", err
	}
	co, err := parseDay(checkOut, "checkOut")
	if err != nil {
		return "", "", err
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if ci.Before(today) {
		return "", "", apperr.BadRequestf("checkIn gecmis bir tarih olamaz")
	}
	if !co.After(ci) {
		return "", "", apperr.BadRequestf("checkOut, checkIn'den sonra olmali")
	}
	if nights := int(co.Sub(ci).Hours() / 24); nights > maxNights {
		return "", "", apperr.BadRequestf("konaklama en fazla %d gece olabilir", maxNights)
	}
	if ci.Sub(today).Hours()/24 > maxAhead {
		return "", "", apperr.BadRequestf("checkIn en fazla %d gun ileride olabilir", maxAhead)
	}
	if adults < 1 || adults > maxAdults {
		return "", "", apperr.BadRequestf("adults 1-%d arasinda olmali", maxAdults)
	}
	if children < 0 || children > maxChildren {
		return "", "", apperr.BadRequestf("children 0-%d arasinda olmali", maxChildren)
	}
	if len(childAges) > 0 && len(childAges) != children {
		return "", "", apperr.BadRequestf("childAges sayisi (%d) children (%d) ile eslesmeli", len(childAges), children)
	}
	for _, a := range childAges {
		if a < 0 || a > 17 {
			return "", "", apperr.BadRequestf("cocuk yasi 0-17 arasinda olmali (%d)", a)
		}
	}
	return ci.Format("2006-01-02"), co.Format("2006-01-02"), nil
}

func validatePriceQuery(q *types.PriceQuery) error {
	if strings.TrimSpace(q.HotelID) == "" {
		return apperr.BadRequestf("hotelId zorunlu")
	}
	ci, co, err := validateStay(q.CheckIn, q.CheckOut, q.Adults, q.Children, q.ChildAges)
	if err != nil {
		return err
	}
	q.CheckIn, q.CheckOut = ci, co
	return nil
}

func validatePackageQuery(q *types.PackageQuery) error {
	if strings.TrimSpace(q.HotelID) == "" {
		return apperr.BadRequestf("hotelId zorunlu")
	}
	if !reIATA.MatchString(strings.TrimSpace(q.AirportCode)) {
		return apperr.BadRequestf("airportCode 3 harfli IATA kodu olmali (ornek IST, SAW)")
	}
	q.AirportCode = strings.ToUpper(strings.TrimSpace(q.AirportCode))
	ci, co, err := validateStay(q.CheckIn, q.CheckOut, q.Adults, q.Children, q.ChildAges)
	if err != nil {
		return err
	}
	q.CheckIn, q.CheckOut = ci, co
	return nil
}

func validateBulkQuery(q *types.BulkPriceQuery) error {
	if len(q.HotelIDs) == 0 {
		return apperr.BadRequestf("hotelIds bos olamaz")
	}
	if len(q.HotelIDs) > 50 {
		return apperr.BadRequestf("hotelIds en fazla 50 otel icerebilir")
	}
	ci, co, err := validateStay(q.CheckIn, q.CheckOut, q.Adults, q.Children, q.ChildAges)
	if err != nil {
		return err
	}
	q.CheckIn, q.CheckOut = ci, co
	return nil
}

// providerNamesValid istenen provider adlarinin kayitli oldugunu dogrular.
func (s *Server) providerNamesValid(names []string) error {
	for _, n := range names {
		if _, err := s.reg.Get(n); err != nil {
			return err
		}
	}
	return nil
}

func boolParam(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "1" || v == "true" || v == "yes"
}

func describe(q types.PriceQuery) string {
	return fmt.Sprintf("%s %s..%s %dA%dC", q.HotelID, q.CheckIn, q.CheckOut, q.Adults, q.Children)
}
