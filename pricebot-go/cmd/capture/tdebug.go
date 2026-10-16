package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"pricebot/internal/browser"
	"pricebot/internal/config"
	"pricebot/internal/providers/touristica"
)

// touristica-debug <detayURL> <checkIn> <checkOut> [checkIn2 checkOut2]
// Gercek havuz akisiyla arama + paket sekmesi adimlarini izler (ikinci tarih verilirse ayni sekmede tekrar).
func touristicaDebug() {
	if len(os.Args) < 5 {
		fmt.Println("kullanim: capture touristica-debug <detayURL> <checkIn> <checkOut> [checkIn2 checkOut2]")
		os.Exit(2)
	}
	cfg := config.Load()
	bm := browser.New(cfg)
	defer bm.Close()
	p := touristica.New(bm, 1, 20*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	for _, l := range p.DebugPackageFlow(ctx, os.Args[2], os.Args[3], os.Args[4]) {
		fmt.Println(l)
	}
	if len(os.Args) >= 7 {
		fmt.Println("=== ikinci tur (ayni sekme)")
		for _, l := range p.DebugPackageFlow(ctx, os.Args[2], os.Args[5], os.Args[6]) {
			fmt.Println(l)
		}
	}
}
