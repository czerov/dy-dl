package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"douyin-nas-monitor/internal/config"
)

const downloadLinePrefix = "DYDL_DOWNLOAD\t"

type Downloader struct{}

type Job struct {
	UserName          string
	UserURL           string
	Quality           string
	SaveDir           string
	CookiesFile       string
	ArchiveFile       string
	YTDLPPath         string
	MergeOutputFormat string
	OutputTemplate    string
}

type Result struct {
	Items  []DownloadedItem
	Output string
}

type DownloadedItem struct {
	ID       string
	Title    string
	FilePath string
}

func New() *Downloader {
	return &Downloader{}
}

func (d *Downloader) Run(ctx context.Context, job Job) (Result, error) {
	args, err := BuildArgs(job)
	if err != nil {
		return Result{}, err
	}

	cmd := exec.CommandContext(ctx, job.YTDLPPath, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	result := Result{
		Items:  ParseDownloadedItems(stdout.String()),
		Output: output,
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if err != nil {
		return result, fmt.Errorf("yt-dlp failed: %w: %s", err, tail(output, 3000))
	}
	return result, nil
}

func BuildArgs(job Job) ([]string, error) {
	format, err := FormatForQuality(job.Quality)
	if err != nil {
		return nil, err
	}
	if job.UserURL == "" {
		return nil, errors.New("user url is required")
	}
	if job.YTDLPPath == "" {
		return nil, errors.New("yt-dlp path is required")
	}

	return []string{
		"--cookies", job.CookiesFile,
		"--download-archive", job.ArchiveFile,
		"--no-overwrites",
		"--continue",
		"--no-progress",
		"--merge-output-format", job.MergeOutputFormat,
		"--print", "after_move:" + downloadLinePrefix + "%(id)s\t%(title)s\t%(filepath)s",
		"-f", format,
		"-o", job.OutputTemplate,
		"-P", job.SaveDir,
		job.UserURL,
	}, nil
}

func FormatForQuality(quality string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(quality)) {
	case "best":
		return "bestvideo+bestaudio/best", nil
	case "1080":
		return "bv*[height<=1080]+ba/b[height<=1080]/best", nil
	case "720":
		return "bv*[height<=720]+ba/b[height<=720]/best", nil
	case "480":
		return "bv*[height<=480]+ba/b[height<=480]/best", nil
	default:
		return "", fmt.Errorf("unsupported quality %q", quality)
	}
}

func ParseDownloadedItems(output string) []DownloadedItem {
	var items []DownloadedItem
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, downloadLinePrefix) {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(line, downloadLinePrefix), "\t", 3)
		if len(parts) != 3 {
			continue
		}
		items = append(items, DownloadedItem{
			ID:       strings.TrimSpace(parts[0]),
			Title:    strings.TrimSpace(parts[1]),
			FilePath: strings.TrimSpace(parts[2]),
		})
	}
	return items
}

func JobFromConfig(cfg config.Config, user config.UserConfig) Job {
	saveDir := user.SaveDir
	if saveDir == "" {
		saveDir = cfg.App.DefaultSaveDir
	}
	return Job{
		UserName:          user.Name,
		UserURL:           user.URL,
		Quality:           user.Quality.String(),
		SaveDir:           saveDir,
		CookiesFile:       cfg.App.CookiesFile,
		ArchiveFile:       cfg.App.ArchiveFile,
		YTDLPPath:         cfg.App.YTDLPPath,
		MergeOutputFormat: cfg.Download.MergeOutputFormat,
		OutputTemplate:    cfg.Download.OutputTemplate,
	}
}

func tail(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[len(value)-max:]
}
