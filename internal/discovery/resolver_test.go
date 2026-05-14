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

func TestDouyinCollectionIDAcceptsMixDetail(t *testing.T) {
	got, ok := DouyinCollectionID("https://www.douyin.com/mix/detail/7234567890123456789")
	if !ok {
		t.Fatal("expected collection URL")
	}
	if got != "7234567890123456789" {
		t.Fatalf("DouyinCollectionID() = %q", got)
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

func TestImportMediaItemsFromBrowserJSON(t *testing.T) {
	content := `{
		"source_url": "https://www.douyin.com/user/MS4wLjABAAAA123",
		"items": [
			{"type": "work", "title": "第一集", "url": "https://www.douyin.com/video/7123456789012345678"},
			{"type": "collection", "title": "合集 A", "url": "/mix/detail/7234567890123456789"},
			{"type": "series", "title": "短剧 A", "id": "7345678901234567890"}
		]
	}`
	got, err := ImportMediaItems("https://www.douyin.com/user/MS4wLjABAAAA123", content)
	if err != nil {
		t.Fatal(err)
	}
	want := []MediaItem{
		{ID: "7123456789012345678", Type: TypeWork, Title: "第一集", URL: "https://www.douyin.com/video/7123456789012345678"},
		{ID: "7234567890123456789", Type: TypeCollection, Title: "合集 A", URL: "https://www.douyin.com/collection/7234567890123456789"},
		{ID: "7345678901234567890", Type: TypeSeries, Title: "短剧 A", URL: "https://www.douyin.com/series/7345678901234567890"},
	}
	if !reflect.DeepEqual(got.Items, want) {
		t.Fatalf("ImportMediaItems() = %#v, want %#v", got.Items, want)
	}
}

func TestImportMediaItemsFromPlainText(t *testing.T) {
	got, err := ImportMediaItems("", "https://www.douyin.com/video/7123456789012345678\nhttps://www.douyin.com/series/7345678901234567890")
	if err != nil {
		t.Fatal(err)
	}
	want := []MediaItem{
		{ID: "7123456789012345678", Type: TypeWork, URL: "https://www.douyin.com/video/7123456789012345678"},
		{ID: "7345678901234567890", Type: TypeSeries, URL: "https://www.douyin.com/series/7345678901234567890"},
	}
	if !reflect.DeepEqual(got.Items, want) {
		t.Fatalf("ImportMediaItems() = %#v, want %#v", got.Items, want)
	}
}

func TestDiscoveryCandidateURLs(t *testing.T) {
	got := discoveryCandidateURLs("https://www.douyin.com/user/MS4wLjABAAAA123?from_tab_name=main")
	if len(got) < 4 {
		t.Fatalf("expected multiple tab candidates, got %#v", got)
	}
	if got[0] != "https://www.douyin.com/user/MS4wLjABAAAA123" {
		t.Fatalf("first candidate = %q", got[0])
	}
	if got[2] != "https://www.douyin.com/user/MS4wLjABAAAA123?showTab=collection" {
		t.Fatalf("collection candidate = %q", got[2])
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
