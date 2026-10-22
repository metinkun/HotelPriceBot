// probe: gelistirme araci. Bir provider'i DB/HTTP sunucusu olmadan dogrudan calistirir.
//
//	probe <provider> resolve <url>
//	probe <provider> price   <url> <checkIn> <checkOut> <adults> [children] [childAges 5,8]
//	probe <provider> package <url> <checkIn> <checkOut> <adults> <airportCode>
//
// URL once resolveFromUrl ile cozulur, cikan mapping ile fiyat/paket sorgusu yapilir.
// Tarayici gerektiren provider'lar icin app imajinda calistirin (bkz. docs/phase3-research.md).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/browser"
	"pricebot/internal/config"
	"pricebot/internal/provider"
	"pricebot/internal/providers"
	"pricebot/internal/types"
)

func die(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "HATA: "+f+"\n", a...)
	os.Exit(1)
}

func dump(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("kullanim: probe <provider> resolve|price|package <url> [checkIn checkOut adults [children] [childAges] | checkIn checkOut adults airportCode]")
		os.Exit(2)
	}
	name, mode, url := os.Args[1], os.Args[2], os.Args[3]
	cfg := config.Load()
	bm := browser.New(cfg)
	defer bm.Close()
	reg := providers.New(cfg, bm)
	p, err := reg.Get(name)
	if err != nil {
		die("%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	rr, ok := providers.ResolverOf(p)
	if !ok {
		die("provider URL cozumlemeyi desteklemiyor")
	}
	t0 := time.Now()
	res, err := rr.ResolveFromURL(ctx, url)
	if err != nil {
		die("resolve: %v", err)
	}
	fmt.Printf("# resolve (%.1fs)\n", time.Since(t0).Seconds())
	dump(res)
	m := &provider.Mapping{Provider: name, InternalHotelID: "probe", ProviderHotelID: res.ProviderHotelID, HotelName: res.HotelName, Metadata: res.Metadata, IsActive: true}
	if mode == "resolve" {
		return
	}
	if len(os.Args) < 7 {
		die("checkIn checkOut adults gerekli")
	}
	ci, co := os.Args[4], os.Args[5]
	adults, _ := strconv.Atoi(os.Args[6])

	switch mode {
	case "price":
		children := 0
		ages := []int{}
		if len(os.Args) > 7 {
			children, _ = strconv.Atoi(os.Args[7])
		}
		if len(os.Args) > 8 {
			for _, s := range strings.Split(os.Args[8], ",") {
				if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
					ages = append(ages, n)
				}
			}
		}
		t1 := time.Now()
		r, err := p.GetPrice(ctx, m, types.PriceQuery{HotelID: "probe", CheckIn: ci, CheckOut: co, Adults: adults, Children: children, ChildAges: ages})
		if err != nil {
			die("price: %v", err)
		}
		fmt.Printf("# price (%.1fs)\n", time.Since(t1).Seconds())
		dump(r)
	case "package":
		if len(os.Args) < 8 {
			die("airportCode gerekli")
		}
		pp, ok := providers.PackageProviderOf(p)
		if !ok {
			die("provider paket desteklemiyor")
		}
		t1 := time.Now()
		r, err := pp.GetPackagePrices(ctx, m, types.PackageQuery{HotelID: "probe", AirportCode: strings.ToUpper(os.Args[7]), CheckIn: ci, CheckOut: co, Adults: adults, ChildAges: []int{}})
		if err != nil {
			die("package: %v", err)
		}
		fmt.Printf("# package (%.1fs)\n", time.Since(t1).Seconds())
		dump(r)
	default:
		die("mod resolve|price|package olmali")
	}
}
