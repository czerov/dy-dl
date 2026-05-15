package downloader

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseDouyinVideoMeta(t *testing.T) {
	page := `<html><script>window._ROUTER_DATA = {
		"loaderData": {
			"video_7022771022038388255/page": {
				"videoInfoRes": {
					"item_list": [{
						"aweme_id": "7022771022038388255",
						"desc": "第一集 / 测试",
						"video": {
							"play_addr": {
								"uri": "v0200fg10000abc",
								"url_list": ["https://example.com/video.mp4"]
							}
						}
					}]
				}
			}
		}
	};</script></html>`

	got, err := parseDouyinVideoMeta("7022771022038388255", page)
	if err != nil {
		t.Fatal(err)
	}
	want := douyinVideoMeta{
		ID:       "7022771022038388255",
		Title:    "第一集 / 测试",
		PlayURI:  "v0200fg10000abc",
		PlayURLs: []string{"https://example.com/video.mp4"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDouyinVideoMeta() = %#v, want %#v", got, want)
	}
}

func TestDirectDouyinDownloadURL(t *testing.T) {
	got, err := directDouyinDownloadURL(douyinVideoMeta{PlayURI: "v0200fg10000abc"}, "720")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://www.iesdouyin.com/aweme/v1/play/?line=0&ratio=720p&video_id=v0200fg10000abc"
	if got != want {
		t.Fatalf("directDouyinDownloadURL() = %q, want %q", got, want)
	}
}

func TestDirectOutputPathSanitizesTitle(t *testing.T) {
	got := directOutputPath(Job{UserName: "风云/笑红尘", SaveDir: "/downloads"}, douyinVideoMeta{
		ID:    "7022771022038388255",
		Title: `第一集: "测试"?`,
	})
	want := filepath.Join("/downloads", "风云_笑红尘", "第一集_ _测试__-7022771022038388255.mp4")
	if got != want {
		t.Fatalf("directOutputPath() = %q, want %q", got, want)
	}
}

func TestDirectArchiveContains(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.txt")
	if err := os.WriteFile(path, []byte("Douyin 111\n7022771022038388255\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := directArchiveContains(path, "7022771022038388255")
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("expected archive hit")
	}
}
