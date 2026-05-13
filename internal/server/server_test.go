package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"douyin-nas-monitor/internal/config"
)

func TestWriteCookieFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.txt")
	content := "# Netscape HTTP Cookie File\n.douyin.com\tTRUE\t/\tTRUE\t0\ttest\tvalue"

	if err := writeCookieFile(path, content); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); !strings.HasSuffix(got, "\n") {
		t.Fatalf("cookie file should end with newline, got %q", got)
	}

	status := cookieStatus(path)
	if !status.Exists {
		t.Fatal("cookie status should report file exists")
	}
	if status.Size == 0 {
		t.Fatal("cookie status should report size")
	}
}

func TestNormalizeRawCookieHeader(t *testing.T) {
	got, err := normalizeCookieContent("Cookie: sessionid=secret; ttwid=abc; is_staff_user=false")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "# Netscape HTTP Cookie File\n") {
		t.Fatalf("expected Netscape header, got %q", got)
	}
	if !strings.Contains(got, ".douyin.com\tTRUE\t/\tTRUE\t2147483647\tsessionid\tsecret") {
		t.Fatalf("expected converted sessionid cookie, got %q", got)
	}
}

func TestHandleLogsRedactsSensitiveValues(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "douyin-monitor.log")
	if err := os.WriteFile(logPath, []byte("sessionid=secret; plain=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := New("", config.Config{
		App: config.AppConfig{LogFile: logPath},
	}, nil, nil, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)

	server.handleLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(resp["text"], "secret") {
		t.Fatalf("log response leaked secret: %q", resp["text"])
	}
	if !strings.Contains(resp["text"], "plain=value") {
		t.Fatalf("log response should keep non-sensitive value: %q", resp["text"])
	}
}
