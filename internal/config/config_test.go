package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSupportsNumericQuality(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
app:
  cookies_file: cookies.txt
users:
  - name: user-a
    url: https://www.douyin.com/user/example
    enabled: true
    quality: 1080
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Users[0].Quality.String(); got != "1080" {
		t.Fatalf("quality = %q, want 1080", got)
	}
	if !filepath.IsAbs(cfg.App.CookiesFile) {
		t.Fatalf("cookies path should be absolute, got %q", cfg.App.CookiesFile)
	}
}

func TestValidateRejectsBadQuality(t *testing.T) {
	cfg := Defaults()
	cfg.Users = []UserConfig{{
		Name:    "user-a",
		URL:     "https://www.douyin.com/user/example",
		Enabled: true,
		Quality: "4k",
	}}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
