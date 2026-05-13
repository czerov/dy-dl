package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
