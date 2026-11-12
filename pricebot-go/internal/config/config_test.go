package config

import "testing"

func TestProductionPublicURLDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PUBLIC_API_URL", "")
	t.Setenv("PUBLIC_PORTAL_URL", "")
	c := Load()
	if c.PublicAPIURL != "https://api.smartrate.app" || c.PublicPortalURL != "https://demo.smartrate.app/portal/" {
		t.Fatalf("production varsayilanlari: api=%q portal=%q", c.PublicAPIURL, c.PublicPortalURL)
	}
	t.Setenv("PUBLIC_API_URL", "https://x.example.com")
	if Load().PublicAPIURL != "https://x.example.com" {
		t.Fatal(".env degeri varsayilani ezmeli")
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("PUBLIC_API_URL", "")
	if Load().PublicAPIURL != "" {
		t.Fatal("development'ta bos kalmali (istekten turetilir)")
	}
}
