package storage

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"os"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type DownloadRecord struct {
	UserName string
	UserURL  string
	VideoID  string
	Title    string
	FilePath string
	Quality  string
	Status   string
	Error    string
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.init(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS downloads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_name TEXT,
    user_url TEXT,
    video_id TEXT UNIQUE,
    title TEXT,
    file_path TEXT,
    quality TEXT,
    status TEXT,
    error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_downloads_user_name ON downloads(user_name);
CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
`)
	return err
}

func (s *Store) UpsertDownload(ctx context.Context, record DownloadRecord) error {
	if record.VideoID == "" {
		return fmt.Errorf("video id is required")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO downloads (user_name, user_url, video_id, title, file_path, quality, status, error, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(video_id) DO UPDATE SET
    user_name = excluded.user_name,
    user_url = excluded.user_url,
    title = excluded.title,
    file_path = excluded.file_path,
    quality = excluded.quality,
    status = excluded.status,
    error = excluded.error,
    updated_at = CURRENT_TIMESTAMP
`, record.UserName, record.UserURL, record.VideoID, record.Title, record.FilePath, record.Quality, record.Status, record.Error)
	return err
}

func FailureID(userURL string, at time.Time) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s|%d", userURL, at.UnixNano())))
	return "failed:" + hex.EncodeToString(sum[:])
}
