package downloader

import "testing"

func TestFormatForQuality(t *testing.T) {
	tests := map[string]string{
		"best": "bestvideo+bestaudio/best",
		"1080": "bv*[height<=1080]+ba/b[height<=1080]/best",
		"720":  "bv*[height<=720]+ba/b[height<=720]/best",
		"480":  "bv*[height<=480]+ba/b[height<=480]/best",
	}
	for quality, want := range tests {
		got, err := FormatForQuality(quality)
		if err != nil {
			t.Fatalf("FormatForQuality(%q): %v", quality, err)
		}
		if got != want {
			t.Fatalf("FormatForQuality(%q) = %q, want %q", quality, got, want)
		}
	}
}

func TestParseDownloadedItems(t *testing.T) {
	output := "noise\nDYDL_DOWNLOAD\tabc123\tA title\t/downloads/a.mp4\n"
	items := ParseDownloadedItems(output)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].ID != "abc123" || items[0].Title != "A title" || items[0].FilePath != "/downloads/a.mp4" {
		t.Fatalf("unexpected item: %#v", items[0])
	}
}

func TestNormalizeUserURLStripsDouyinTrackingParams(t *testing.T) {
	input := " https://www.douyin.com/user/MS4wLjABAAAA123?from_tab_name=main&is_search=0#profile "
	want := "https://www.douyin.com/user/MS4wLjABAAAA123"
	if got := NormalizeUserURL(input); got != want {
		t.Fatalf("NormalizeUserURL() = %q, want %q", got, want)
	}
}

func TestNormalizeUserURLKeepsNonDouyinURL(t *testing.T) {
	input := "https://example.com/user/abc?keep=1"
	if got := NormalizeUserURL(input); got != input {
		t.Fatalf("NormalizeUserURL() = %q, want %q", got, input)
	}
}
