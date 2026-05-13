package monitor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"douyin-nas-monitor/internal/config"
)

type CheckResult struct {
	Name    string
	OK      bool
	Message string
}

func CheckEnvironment(ctx context.Context, cfg config.Config) []CheckResult {
	results := []CheckResult{
		checkFile("cookies.txt", cfg.App.CookiesFile),
		checkWritablePath("database directory", cfg.App.Database),
		checkWritablePath("archive directory", cfg.App.ArchiveFile),
		checkWritablePath("log directory", cfg.App.LogFile),
		checkWritableDir("download directory", cfg.App.DefaultSaveDir),
		checkCommand(ctx, "yt-dlp", cfg.App.YTDLPPath),
		checkCommand(ctx, "ffmpeg", "ffmpeg"),
	}

	enabled := 0
	for _, user := range cfg.Users {
		if user.Enabled {
			enabled++
		}
	}
	results = append(results, CheckResult{
		Name: "enabled users",
		OK:   enabled > 0,
		Message: func() string {
			if enabled > 0 {
				return ""
			}
			return "no enabled users configured"
		}(),
	})
	return results
}

func checkFile(name, path string) CheckResult {
	if path == "" {
		return CheckResult{Name: name, OK: false, Message: "path is empty"}
	}
	info, err := os.Stat(path)
	if err != nil {
		return CheckResult{Name: name, OK: false, Message: err.Error()}
	}
	if info.IsDir() {
		return CheckResult{Name: name, OK: false, Message: "path is a directory"}
	}
	return CheckResult{Name: name, OK: true}
}

func checkWritablePath(name, path string) CheckResult {
	if path == "" {
		return CheckResult{Name: name, OK: false, Message: "path is empty"}
	}
	return checkWritableDir(name, filepath.Dir(path))
}

func checkWritableDir(name, dir string) CheckResult {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return CheckResult{Name: name, OK: false, Message: err.Error()}
	}
	temp, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return CheckResult{Name: name, OK: false, Message: err.Error()}
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		return CheckResult{Name: name, OK: false, Message: err.Error()}
	}
	if err := os.Remove(tempPath); err != nil {
		return CheckResult{Name: name, OK: false, Message: err.Error()}
	}
	return CheckResult{Name: name, OK: true}
}

func checkCommand(ctx context.Context, name, command string) CheckResult {
	if command == "" {
		return CheckResult{Name: name, OK: false, Message: "command is empty"}
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return CheckResult{Name: name, OK: false, Message: err.Error()}
	}
	cmd := exec.CommandContext(ctx, path, "--version")
	if err := cmd.Run(); err != nil {
		return CheckResult{Name: name, OK: false, Message: fmt.Sprintf("%s exists but --version failed: %v", path, err)}
	}
	return CheckResult{Name: name, OK: true}
}
