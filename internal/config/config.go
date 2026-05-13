package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ModeOnce   = "once"
	ModeDaemon = "daemon"
)

type Config struct {
	App      AppConfig      `yaml:"app" json:"app"`
	Download DownloadConfig `yaml:"download" json:"download"`
	Users    []UserConfig   `yaml:"users" json:"users"`
	Notify   NotifyConfig   `yaml:"notify" json:"notify"`
}

type AppConfig struct {
	Mode                     string `yaml:"mode" json:"mode"`
	IntervalMinutes          int    `yaml:"interval_minutes" json:"interval_minutes"`
	SleepBetweenUsersSeconds int    `yaml:"sleep_between_users_seconds" json:"sleep_between_users_seconds"`
	LogFile                  string `yaml:"log_file" json:"log_file"`
	Database                 string `yaml:"database" json:"database"`
	CookiesFile              string `yaml:"cookies_file" json:"cookies_file"`
	ArchiveFile              string `yaml:"archive_file" json:"archive_file"`
	DefaultSaveDir           string `yaml:"default_save_dir" json:"default_save_dir"`
	YTDLPPath                string `yaml:"yt_dlp_path" json:"yt_dlp_path"`
	TimeoutSeconds           int    `yaml:"timeout_seconds" json:"timeout_seconds"`
}

type DownloadConfig struct {
	MergeOutputFormat string `yaml:"merge_output_format" json:"merge_output_format"`
	OutputTemplate    string `yaml:"output_template" json:"output_template"`
	Retries           int    `yaml:"retries" json:"retries"`
}

type UserConfig struct {
	Name    string  `yaml:"name" json:"name"`
	URL     string  `yaml:"url" json:"url"`
	Enabled bool    `yaml:"enabled" json:"enabled"`
	Quality Quality `yaml:"quality" json:"quality"`
	SaveDir string  `yaml:"save_dir" json:"save_dir"`
}

type NotifyConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Type       string `yaml:"type" json:"type"`
	WebhookURL string `yaml:"webhook_url" json:"webhook_url"`
}

type Quality string

func (q *Quality) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		*q = Quality(strings.TrimSpace(value.Value))
		return nil
	default:
		return fmt.Errorf("quality must be a scalar")
	}
}

func (q *Quality) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*q = Quality(strings.TrimSpace(str))
		return nil
	}

	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		*q = Quality(strings.TrimSpace(number.String()))
		return nil
	}

	return fmt.Errorf("quality must be a string or number")
}

func (q Quality) String() string {
	return string(q)
}

func Load(path string) (Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	if abs, err := filepath.Abs(path); err == nil {
		cfg = cfg.WithRelativePaths(filepath.Dir(abs))
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func Defaults() Config {
	return Config{
		App: AppConfig{
			Mode:                     ModeOnce,
			IntervalMinutes:          120,
			SleepBetweenUsersSeconds: 30,
			LogFile:                  "logs/douyin-monitor.log",
			Database:                 "data/douyin-monitor.db",
			CookiesFile:              "cookies.txt",
			ArchiveFile:              "data/archive.txt",
			DefaultSaveDir:           "downloads",
			YTDLPPath:                "yt-dlp",
			TimeoutSeconds:           1800,
		},
		Download: DownloadConfig{
			MergeOutputFormat: "mp4",
			OutputTemplate:    "%(uploader)s/%(upload_date)s-%(title).80s-%(id)s.%(ext)s",
			Retries:           3,
		},
		Notify: NotifyConfig{
			Type: "generic",
		},
	}
}

func (cfg Config) Validate() error {
	var errs []error

	if cfg.App.Mode == "" {
		cfg.App.Mode = ModeOnce
	}
	if cfg.App.Mode != ModeOnce && cfg.App.Mode != ModeDaemon {
		errs = append(errs, fmt.Errorf("app.mode must be %q or %q", ModeOnce, ModeDaemon))
	}
	if cfg.App.IntervalMinutes <= 0 {
		errs = append(errs, errors.New("app.interval_minutes must be greater than 0"))
	}
	if cfg.App.SleepBetweenUsersSeconds < 0 {
		errs = append(errs, errors.New("app.sleep_between_users_seconds cannot be negative"))
	}
	if cfg.App.LogFile == "" {
		errs = append(errs, errors.New("app.log_file is required"))
	}
	if cfg.App.Database == "" {
		errs = append(errs, errors.New("app.database is required"))
	}
	if cfg.App.CookiesFile == "" {
		errs = append(errs, errors.New("app.cookies_file is required"))
	}
	if cfg.App.ArchiveFile == "" {
		errs = append(errs, errors.New("app.archive_file is required"))
	}
	if cfg.App.DefaultSaveDir == "" {
		errs = append(errs, errors.New("app.default_save_dir is required"))
	}
	if cfg.App.YTDLPPath == "" {
		errs = append(errs, errors.New("app.yt_dlp_path is required"))
	}
	if cfg.App.TimeoutSeconds <= 0 {
		errs = append(errs, errors.New("app.timeout_seconds must be greater than 0"))
	}
	if cfg.Download.MergeOutputFormat == "" {
		errs = append(errs, errors.New("download.merge_output_format is required"))
	}
	if cfg.Download.OutputTemplate == "" {
		errs = append(errs, errors.New("download.output_template is required"))
	}
	if cfg.Download.Retries < 0 {
		errs = append(errs, errors.New("download.retries cannot be negative"))
	}
	for i, user := range cfg.Users {
		prefix := "users[" + strconv.Itoa(i) + "]"
		if strings.TrimSpace(user.Name) == "" {
			errs = append(errs, fmt.Errorf("%s.name is required", prefix))
		}
		if strings.TrimSpace(user.URL) == "" {
			errs = append(errs, fmt.Errorf("%s.url is required", prefix))
		}
		if !IsValidQuality(user.Quality.String()) {
			errs = append(errs, fmt.Errorf("%s.quality must be one of best, 1080, 720, 480", prefix))
		}
	}
	if cfg.Notify.Enabled {
		if cfg.Notify.Type == "" {
			errs = append(errs, errors.New("notify.type is required when notify.enabled is true"))
		}
		if cfg.Notify.WebhookURL == "" {
			errs = append(errs, errors.New("notify.webhook_url is required when notify.enabled is true"))
		}
	}

	return errors.Join(errs...)
}

func IsValidQuality(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "best", "1080", "720", "480":
		return true
	default:
		return false
	}
}

func (cfg Config) WithRelativePaths(baseDir string) Config {
	cfg.App.LogFile = resolvePath(baseDir, cfg.App.LogFile)
	cfg.App.Database = resolvePath(baseDir, cfg.App.Database)
	cfg.App.CookiesFile = resolvePath(baseDir, cfg.App.CookiesFile)
	cfg.App.ArchiveFile = resolvePath(baseDir, cfg.App.ArchiveFile)
	cfg.App.DefaultSaveDir = resolvePath(baseDir, cfg.App.DefaultSaveDir)
	for i := range cfg.Users {
		cfg.Users[i].SaveDir = resolvePath(baseDir, cfg.Users[i].SaveDir)
	}
	return cfg
}

func resolvePath(baseDir, value string) string {
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(baseDir, value)
}
