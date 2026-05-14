package discovery

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeSourceURLStripsDouyinParams(t *testing.T) {
	input := " https://www.douyin.com/user/MS4wLjABAAAA123?from_tab_name=main#profile "
	want := "https://www.douyin.com/user/MS4wLjABAAAA123"
	if got := NormalizeSourceURL(input); got != want {
		t.Fatalf("NormalizeSourceURL() = %q, want %q", got, want)
	}
}

func TestDouyinVideoID(t *testing.T) {
	got, ok := DouyinVideoID("https://www.douyin.com/video/7123456789012345678?previous_page=app")
	if !ok {
		t.Fatal("expected video URL")
	}
	if got != "7123456789012345678" {
		t.Fatalf("DouyinVideoID() = %q", got)
	}
}

func TestExtractVideoIDs(t *testing.T) {
	page := `%7B%22aweme_id%22%3A%227123456789012345678%22%2C%22url%22%3A%22https%3A%5C%2F%5C%2Fwww.douyin.com%5C%2Fvideo%5C%2F7987654321098765432%22%7D`
	want := []string{"7987654321098765432", "7123456789012345678"}
	if got := ExtractVideoIDs(page); !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractVideoIDs() = %#v, want %#v", got, want)
	}
}

func TestExtractMediaItems(t *testing.T) {
	page := `
		<a href="/video/7123456789012345678">作品</a>
		<a href="/collection/7234567890123456789">合集</a>
		{"series_id":"7345678901234567890"}
	`
	got := ExtractMediaItems(page)
	want := []MediaItem{
		{ID: "7123456789012345678", Type: TypeWork, URL: "https://www.douyin.com/video/7123456789012345678"},
		{ID: "7234567890123456789", Type: TypeCollection, URL: "https://www.douyin.com/collection/7234567890123456789"},
		{ID: "7345678901234567890", Type: TypeSeries, URL: "https://www.douyin.com/series/7345678901234567890"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractMediaItems() = %#v, want %#v", got, want)
	}
}

func TestCookieHeaderFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.txt")
	content := "# Netscape HTTP Cookie File\n.douyin.com\tTRUE\t/\tTRUE\t2147483647\tsessionid\tabc\n.douyin.com\tTRUE\t/\tTRUE\t2147483647\tttwid\tdef\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := CookieHeaderFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "sessionid=abc; ttwid=def"
	if got != want {
		t.Fatalf("CookieHeaderFromFile() = %q, want %q", got, want)
	}
}
