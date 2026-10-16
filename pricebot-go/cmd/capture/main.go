// capture: gelistirme araci. Provider'larin ham yanitlarini test fixture'i olarak kaydeder.
// Kullanim: go run ./cmd/capture tatilbudur <detayURL> <hotelId> <checkIn> <checkOut> <outDir>
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"pricebot/internal/httpx"
)

const ua = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"

var pageHeaders = map[string]string{
	"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"accept-language":           "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"user-agent":                ua,
	"sec-ch-ua":                 `"Chromium";v="150", "Google Chrome";v="150", "Not;A=Brand";v="8"`,
	"sec-ch-ua-mobile":          "?0",
	"sec-ch-ua-platform":        `"macOS"`,
	"sec-fetch-dest":            "document",
	"sec-fetch-mode":            "navigate",
	"sec-fetch-site":            "none",
	"upgrade-insecure-requests": "1",
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "HATA:", err)
		os.Exit(1)
	}
}

func save(dir, name string, data []byte) {
	must(os.WriteFile(filepath.Join(dir, name), data, 0o644))
	fmt.Printf("%s: %d bayt\n", name, len(data))
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "jolly" {
		captureJolly()
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "net" {
		captureNet()
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "js" {
		runJS()
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "touristica-debug" {
		touristicaDebug()
		return
	}
	if len(os.Args) < 7 || os.Args[1] != "tatilbudur" {
		fmt.Println("kullanim: capture tatilbudur <detayURL> <hotelId> <checkIn yyyy-MM-dd> <checkOut> <outDir>")
		fmt.Println("         capture jolly <detayURL> <jollyId> <hotelType> <checkIn yyyy.MM.dd> <checkOut> <outDir>")
		os.Exit(2)
	}
	detailURL, hotelID, ci, co, out := os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6]
	ctx := context.Background()

	resp, err := httpx.Do(ctx, http.MethodGet, detailURL, httpx.Options{Headers: pageHeaders, Timeout: 25 * time.Second})
	must(err)
	html := resp.Text()
	token := regexp.MustCompile(`name="_token"\s+value="([^"]+)"`).FindStringSubmatch(html)
	if token == nil {
		must(fmt.Errorf("_token bulunamadi"))
	}
	cookie := resp.SetCookies()
	to := regexp.MustCompile(`&to=([A-Z]{3})&from=`).FindStringSubmatch(html)

	// Detay sayfasinin yalnizca resolve icin gereken parcalari
	var snippet strings.Builder
	for _, re := range []string{`<h1[^>]*hotel-name[^>]*>[^<]*`, `hotelId=\d+`, `&to=[A-Z]{3}&from=`, `name="_token"\s+value="[^"]+"`} {
		if m := regexp.MustCompile(re).FindString(html); m != "" {
			snippet.WriteString(m + "\n")
		}
	}
	save(out, "detail_snippet.html", []byte(snippet.String()))

	p := strings.Split(ci, "-")
	q := strings.Split(co, "-")
	form := httpx.FormValues{
		{"_token", token[1]}, {"productType", "hotel"}, {"hotelId", hotelID}, {"selectedRoom", ""}, {"selectedPricingId", ""},
		{"selectedMealType", ""}, {"hotelOldId", ""}, {"productLoc", ""}, {"productTypeId", ""}, {"code", ""}, {"alertRoom", "0"},
		{"actionPricingId", ""}, {"currencyId", ""}, {"actions[]", "0"}, {"selectedActionCategory", ""}, {"hidePrice", "0"},
		{"googleRemarketingCategory", ""}, {"autoPost", "0"}, {"isFlightPackage", "0"}, {"adult", "2"}, {"child", "0"},
		{"isCyprusPackageManual", "0"}, {"priceConfig", ""}, {"loyaltyPoint", "0"},
		{"daterange-1", fmt.Sprintf("%s/%s/%s - %s/%s/%s", p[2], p[1], p[0], q[2], q[1], q[0])},
		{"checkInDate", fmt.Sprintf("%s.%s.%s", strings.TrimLeft(p[2], "0"), strings.TrimLeft(p[1], "0"), p[0])},
		{"checkOutDate", fmt.Sprintf("%s.%s.%s", strings.TrimLeft(q[2], "0"), strings.TrimLeft(q[1], "0"), q[0])},
		{"quickPersonCount", "2 Yetişkin"}, {"type", ""},
	}
	hdr := map[string]string{
		"user-agent": ua, "x-requested-with": "XMLHttpRequest", "x-csrf-token": token[1],
		"content-type": "application/x-www-form-urlencoded; charset=UTF-8", "accept": "*/*",
		"origin": "https://www.tatilbudur.com", "referer": detailURL, "cookie": cookie,
	}
	r1, err := httpx.Post(ctx, "https://www.tatilbudur.com/hotel/calculate-room-price", []byte(form.Encode()), hdr, 30*time.Second)
	must(err)
	save(out, "room_price.json", r1.Body)

	if to != nil {
		body := form.Encode() + "&to=" + to[1] + "&from=SAW"
		r2, err := httpx.Post(ctx, "https://www.tatilbudur.com/hotel/calculate-package-room-price", []byte(body), hdr, 45*time.Second)
		must(err)
		save(out, "package_price.json", r2.Body)
	}
}

// captureJolly: detay snippet + oda fiyati + paket (SAW=64118) yanitlari.
func captureJolly() {
	if len(os.Args) < 8 {
		fmt.Println("kullanim: capture jolly <detayURL> <jollyId> <hotelType> <checkIn yyyy.MM.dd> <checkOut> <outDir>")
		os.Exit(2)
	}
	detailURL, id, hotelType, ci, co, out := os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6], os.Args[7]
	ctx := context.Background()

	resp, err := httpx.Do(ctx, http.MethodGet, detailURL, httpx.Options{Headers: pageHeaders, Timeout: 20 * time.Second})
	must(err)
	html := resp.Text()
	var snippet strings.Builder
	for _, re := range []string{`name="SearchParameters\.Id"[^>]*value="[^"]*"`, `name="SearchParameters\.HotelType"[^>]*value="[^"]*"`, `name="SearchParameters\.HotelName"[^>]*value="[^"]*"`} {
		if m := regexp.MustCompile(re).FindString(html); m != "" {
			snippet.WriteString("<input " + m + " />\n")
		}
	}
	save(out, "detail_snippet.html", []byte(snippet.String()))

	hdr := map[string]string{
		"accept": "application/json", "accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
		"content-type": "application/x-www-form-urlencoded; charset=UTF-8", "origin": "https://www.jollytur.com",
		"user-agent": ua, "x-requested-with": "XMLHttpRequest", "referer": "https://www.jollytur.com/",
	}
	base := func(pkg bool) httpx.FormValues {
		originID, originType, originName, pst := "", "Zone", "", ""
		if pkg {
			originID, originType, originName, pst = "64118", "Airport", "SAW", "0"
		}
		return httpx.FormValues{
			{"id", id}, {"startDate", ci}, {"endDate", co}, {"rooms", "2"},
			{"originId", originID}, {"originType", originType}, {"originName", originName},
			{"hotelType", hotelType}, {"searchType", "Product"}, {"customerTrackId", "00000000-0000-0000-0000-000000000000"},
			{"packageSearchType", pst}, {"trivagoReferenceId", ""}, {"neredeKalReferenceId", ""}, {"hadsKey", ""}, {"utmSource", ""},
		}
	}
	r1, err := httpx.Post(ctx, "https://www.jollytur.com/hotel/GetReservationCompletePartial", []byte(base(false).Encode()), hdr, 30*time.Second)
	must(err)
	save(out, "reservation.json", r1.Body)
	r2, err := httpx.Post(ctx, "https://www.jollytur.com/hotel/GetReservationCompletePartial", []byte(base(true).Encode()), hdr, 30*time.Second)
	must(err)
	save(out, "package.json", r2.Body)
}
