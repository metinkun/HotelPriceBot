// Package version derleme zamaninda ldflags ile doldurulan surum bilgisini tutar.
//
//	go build -ldflags "-X pricebot/internal/version.Version=1.1.0 -X pricebot/internal/version.Commit=abc1234 -X pricebot/internal/version.BuiltAt=2026-11-12T10:00:00Z"
//
// Dockerfile bunu VERSION dosyasi ve GIT_COMMIT build arg'indan otomatik yapar; elle derlemede "dev" gorunur.
package version

var (
	Version = "dev"
	Commit  = ""
	BuiltAt = ""
)

// Info /health ve arayuz icin JSON'a uygun ozet.
func Info() map[string]string {
	return map[string]string{"version": Version, "commit": Commit, "builtAt": BuiltAt}
}

// String "1.1.0 (abc1234)" biciminde kisa gosterim.
func String() string {
	if Commit == "" {
		return Version
	}
	return Version + " (" + Commit + ")"
}
