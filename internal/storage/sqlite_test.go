package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestUpsertDownload(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "downloads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	record := DownloadRecord{
		UserName: "user-a",
		UserURL:  "https://www.douyin.com/user/example",
		VideoID:  "video-1",
		Title:    "title",
		FilePath: "/downloads/video.mp4",
		Quality:  "1080",
		Status:   "success",
	}
	if err := store.UpsertDownload(context.Background(), record); err != nil {
		t.Fatal(err)
	}

	record.Title = "new title"
	if err := store.UpsertDownload(context.Background(), record); err != nil {
		t.Fatal(err)
	}
}
