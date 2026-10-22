// PriceBot: birlesik cok-saglayicili otel fiyat botu (Go surumu).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"pricebot/internal/api"
	"pricebot/internal/browser"
	"pricebot/internal/cache"
	"pricebot/internal/canary"
	"pricebot/internal/config"
	"pricebot/internal/db"
	"pricebot/internal/jobs"
	"pricebot/internal/limiter"
	"pricebot/internal/providers"
	"pricebot/internal/proxy"
	"pricebot/internal/services"
)

func setupLogger(cfg *config.Config) {
	var h slog.Handler
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if cfg.LogFormat == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(h))
}

func main() {
	cfg := config.Load()
	setupLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("DB baglantisi basarisiz", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	rc := cache.New(ctx, cfg.RedisURL, cfg.CacheTTL)
	limits, def := limiter.ParseLimits(cfg.ProviderConcurrency)
	lim := limiter.New(limits, def, cfg.BreakerFailures, cfg.BreakerOpenFor)
	fetcher := services.NewFetcher(rc, lim)
	usage := services.NewUsage(database, rc)
	fetcher.SetUsageHook(usage.Hook)
	api.BootstrapAdmin(ctx, database, cfg.AdminEmail, cfg.AdminPassword)
	api.StartLogRetention(ctx, database, cfg.LogRetentionDays)

	bm := browser.New(cfg)
	defer bm.Close()

	reg := providers.New(cfg, bm)

	proxySvc := proxy.New(cfg, database)
	proxySvc.Start(ctx)
	defer proxySvc.Stop()

	priceSvc := services.NewPriceService(database, reg, fetcher)
	pkgSvc := services.NewPackageService(database, reg, fetcher)
	kn := canary.New(cfg, priceSvc, rc)
	kn.Start()
	defer kn.Stop()

	jm := jobs.New(rc, priceSvc, pkgSvc, cfg.JobResultTTL, cfg.JobMaxQueued, 16)
	defer jm.Stop()

	apiSrv := api.New(cfg, database, reg, api.Deps{Cache: rc, Limiter: lim, Fetcher: fetcher, Canary: kn, Jobs: jm, Usage: usage})
	apiSrv.Portal().Start() // yarim aramalari kapat + planli arama zamanlayicisi
	defer apiSrv.Portal().Stop()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           apiSrv.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      5 * time.Minute, // Setur/Touristica uzun surebilir
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}

	go func() {
		slog.Info("PriceBot calisiyor", "port", cfg.Port, "env", cfg.Env, "providers", strings.Join(reg.Names(), ","),
			"cache", rc.Enabled(), "swagger", cfg.SwaggerOn)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP sunucu hatasi", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Kapatiliyor...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
