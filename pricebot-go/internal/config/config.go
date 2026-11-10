package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config uygulamanin tum ortam degiskenlerini tasir.
type Config struct {
	Env         string // development | production
	Port        int
	DatabaseURL string
	RedisURL    string
	AdminAPIKey string
	LogFormat   string // json | text
	CORSOrigins []string
	SwaggerOn   bool

	EtsturCookie                 string
	TatilSepetiCookie            string
	TatilSepetiVerificationToken string

	ProxyHost     string
	ProxyPort     int
	ProxyUsername string
	ProxyPassword string

	WebshareAPIKey    string
	BrowserProxyURL   string // headless tarayici (Touristica) icin tek proxy: http://user:pass@host:port

	// E-posta (sifre sifirlama linki). SMTP_HOST bos ise gonderim yapilmaz, link ekranda gosterilir.
	SMTPHost, SMTPUser, SMTPPassword, SMTPFrom, SMTPTLS string
	SMTPPort                                            int
	PublicPortalURL                                     string        // orn. https://app.smartrate.com/portal/  (bos: istekten turetilir)
	PasswordResetTTL                                    time.Duration // link gecerlilik suresi
	ProxySyncInterval time.Duration

	BrowserHeadless       bool
	BrowserExecutablePath string
	BrowserMaxTabs        int
	BrowserRestartAfter   int
	TouristicaTabs        int           // kalici sekme sayisi (= eszamanli Touristica sorgusu)
	TouristicaSessionAge  time.Duration // sekmedeki sayfanin yenilenme suresi

	CacheTTL            time.Duration
	ProviderConcurrency string // "touristica=1,setur=3,default=5"
	BreakerFailures     int
	BreakerOpenFor      time.Duration

	CanaryHotelID  string
	CanaryInterval time.Duration

	JobResultTTL time.Duration
	JobMaxQueued int

	DetailsRunHour   int    // otel statik detayi gunluk kontrol saati (Istanbul)
	RequireAPIKey    bool   // musteri uclari API anahtari ister (prod'da true)
	JWTSecret        string // admin oturumu imzasi
	AdminEmail       string // ilk admin (bootstrap)
	AdminPassword    string
	LogRetentionDays int
	WebAuthnRPID     string   // bos = istekteki host
	WebAuthnOrigins  []string // bos = istekteki Origin
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := env(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(env(key, ""))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes"
}

func envList(key string) []string {
	out := []string{}
	for _, p := range strings.Split(env(key, ""), ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Load .env dosyasini (varsa) okur ve Config olusturur.
func Load() *Config {
	_ = godotenv.Load()

	exe := env("BROWSER_EXECUTABLE_PATH", "")
	if exe == "" {
		exe = env("PUPPETEER_EXECUTABLE_PATH", "")
	}
	envName := env("APP_ENV", env("ENV", "development"))
	logFormat := env("LOG_FORMAT", "")
	if logFormat == "" {
		if envName == "production" {
			logFormat = "json"
		} else {
			logFormat = "text"
		}
	}

	return &Config{
		Env:         envName,
		Port:        envInt("PORT", 3000),
		DatabaseURL: env("DATABASE_URL", "postgresql://botuser:botpass123@localhost:5434/pricebot"),
		RedisURL:    env("REDIS_URL", "redis://localhost:6379/0"),
		AdminAPIKey: env("ADMIN_API_KEY", ""),
		LogFormat:   logFormat,
		CORSOrigins: envList("CORS_ORIGINS"),
		SwaggerOn:   envBool("SWAGGER_ENABLED", envName != "production"),

		EtsturCookie:                 env("ETSTUR_COOKIE", ""),
		TatilSepetiCookie:            env("TATILSEPETI_COOKIE", ""),
		TatilSepetiVerificationToken: env("TATILSEPETI_VERIFICATION_TOKEN", ""),

		ProxyHost:     env("PROXY_HOST", ""),
		ProxyPort:     envInt("PROXY_PORT", 0),
		ProxyUsername: env("PROXY_USERNAME", ""),
		ProxyPassword: env("PROXY_PASSWORD", ""),

		WebshareAPIKey:    env("WEBSHARE_API_KEY", ""),
		ProxySyncInterval: time.Duration(envInt("PROXY_SYNC_INTERVAL_MS", 300000)) * time.Millisecond,
		BrowserProxyURL:   env("BROWSER_PROXY_URL", ""),

		SMTPHost: env("SMTP_HOST", ""), SMTPPort: envInt("SMTP_PORT", 587), SMTPUser: env("SMTP_USER", ""), SMTPPassword: env("SMTP_PASSWORD", ""),
		SMTPFrom: env("SMTP_FROM", ""), SMTPTLS: env("SMTP_TLS", ""),
		PublicPortalURL:  env("PUBLIC_PORTAL_URL", ""),
		PasswordResetTTL: time.Duration(envInt("PASSWORD_RESET_TTL_MINUTES", 60)) * time.Minute,

		BrowserHeadless:       env("PUPPETEER_HEADLESS", "true") != "false",
		BrowserExecutablePath: exe,
		BrowserMaxTabs:        envInt("BROWSER_MAX_TABS", 2),
		BrowserRestartAfter:   envInt("BROWSER_RESTART_AFTER", 25),
		TouristicaTabs:        envInt("TOURISTICA_TABS", 1),
		TouristicaSessionAge:  time.Duration(envInt("TOURISTICA_SESSION_MINUTES", 20)) * time.Minute,

		CacheTTL:            time.Duration(envInt("CACHE_TTL_SECONDS", 300)) * time.Second,
		ProviderConcurrency: env("PROVIDER_CONCURRENCY", "touristica=1,setur=3,default=5"),
		BreakerFailures:     envInt("BREAKER_FAILURES", 5),
		BreakerOpenFor:      time.Duration(envInt("BREAKER_OPEN_SECONDS", 120)) * time.Second,

		CanaryHotelID:  env("CANARY_HOTEL_ID", ""),
		CanaryInterval: time.Duration(envInt("CANARY_INTERVAL_MINUTES", 1440)) * time.Minute,

		JobResultTTL: time.Duration(envInt("JOB_RESULT_TTL_SECONDS", 3600)) * time.Second,
		JobMaxQueued: envInt("JOB_MAX_QUEUED", 200),

		DetailsRunHour:   envInt("DETAILS_RUN_HOUR", 4),
		WebAuthnRPID:     env("WEBAUTHN_RPID", ""),
		WebAuthnOrigins:  envList("WEBAUTHN_ORIGINS"),
		RequireAPIKey:    envBool("REQUIRE_API_KEY", envName == "production"),
		JWTSecret:        env("JWT_SECRET", ""),
		AdminEmail:       env("ADMIN_EMAIL", ""),
		AdminPassword:    env("ADMIN_PASSWORD", ""),
		LogRetentionDays: envInt("LOG_RETENTION_DAYS", 30),
	}
}

func (c *Config) IsProd() bool { return c.Env == "production" }
