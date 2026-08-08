package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("PORT", "8081")
	t.Setenv("CORS_ORIGIN", "https://example.com")
	t.Setenv("PUBLIC_DIR", "assets")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8081 || cfg.CORSOrigin != "https://example.com" || cfg.StaticDir != "assets" {
		t.Fatalf("Load() = %+v, want configured values", cfg)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	for _, port := range []string{"not-a-number", "0", "65536"} {
		t.Run(port, func(t *testing.T) {
			t.Setenv("PORT", port)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
		})
	}

}
