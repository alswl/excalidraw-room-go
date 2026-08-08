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

func TestLoadDefaultMaxHTTPBufferSize(t *testing.T) {
	t.Setenv("MAX_HTTP_BUFFER_SIZE", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MaxHTTPBufferSize != DefaultMaxHTTPBufferSize {
		t.Fatalf("MaxHTTPBufferSize = %d, want default %d", cfg.MaxHTTPBufferSize, DefaultMaxHTTPBufferSize)
	}
}

func TestLoadOverridesMaxHTTPBufferSize(t *testing.T) {
	t.Setenv("MAX_HTTP_BUFFER_SIZE", "5242880")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MaxHTTPBufferSize != 5242880 {
		t.Fatalf("MaxHTTPBufferSize = %d, want 5242880", cfg.MaxHTTPBufferSize)
	}
}

func TestLoadRejectsInvalidMaxHTTPBufferSize(t *testing.T) {
	for _, v := range []string{"not-a-number", "0", "-1"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("MAX_HTTP_BUFFER_SIZE", v)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
		})
	}
}
