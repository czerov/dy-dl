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
