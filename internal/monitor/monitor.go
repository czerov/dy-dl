package monitor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"douyin-nas-monitor/internal/archive"
	"douyin-nas-monitor/internal/config"
	"douyin-nas-monitor/internal/discovery"
	"douyin-nas-monitor/internal/downloader"
	"douyin-nas-monitor/internal/logger"
	"douyin-nas-monitor/internal/notify"
	"douyin-nas-monitor/internal/sensitive"
	"douyin-nas-monitor/internal/storage"
)

type Runner struct {
	cfg        config.Config
	log        *logger.Logger
	store      *storage.Store
	downloader *downloader.Downloader
	notifier   notify.Notifier
}

func NewRunner(cfg config.Config, log *logger.Logger, store *storage.Store, dl *downloader.Downloader, notifier notify.Notifier) *Runner {
	return &Runner{
		cfg:        cfg,
		log:        log,
		store:      store,
		downloader: dl,
		notifier:   notifier,
	}
}

func (r *Runner) RunDaemon(ctx context.Context) error {
	r.log.Infof("starting daemon mode, interval=%d minutes", r.cfg.App.IntervalMinutes)
	for {
		if err := r.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			r.log.Errorf("run once failed: %v", err)
		}

		select {
		case <-ctx.Done():
			r.log.Infof("daemon stopped")
			return nil
		case <-time.After(time.Duration(r.cfg.App.IntervalMinutes) * time.Minute):
		}
	}
}

func (r *Runner) RunOnce(ctx context.Context) error {
	r.log.Infof("starting run")
	if err := r.prepareRuntime(); err != nil {
		return err
	}
	if err := archive.EnsureFile(r.cfg.App.ArchiveFile); err != nil {
		return fmt.Errorf("ensure archive file: %w", err)
	}

	enabledUsers := r.enabledUsers()
	if len(enabledUsers) == 0 {
		r.log.Warnf("no enabled users configured")
		return nil
	}

	var runErr error
	for i, user := range enabledUsers {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := r.processUser(ctx, user); err != nil {
			runErr = errors.Join(runErr, err)
		}
		if i < len(enabledUsers)-1 && r.cfg.App.SleepBetweenUsersSeconds > 0 {
			r.log.Infof("sleeping %d seconds before next user", r.cfg.App.SleepBetweenUsersSeconds)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(r.cfg.App.SleepBetweenUsersSeconds) * time.Second):
			}
		}
	}

	r.log.Infof("run finished")
	return runErr
}

func (r *Runner) prepareRuntime() error {
	info, err := os.Stat(r.cfg.App.CookiesFile)
	if err != nil {
		return fmt.Errorf("cookies.txt 不存在，请先导出自己的抖音账号 Cookie，并保存到 %s: %w", r.cfg.App.CookiesFile, err)
	}
	if info.IsDir() {
		return fmt.Errorf("cookies path is a directory: %s", r.cfg.App.CookiesFile)
	}
	if err := os.MkdirAll(r.cfg.App.DefaultSaveDir, 0o755); err != nil {
		return fmt.Errorf("prepare default download directory: %w", err)
	}
	for _, user := range r.cfg.Users {
		if user.SaveDir == "" {
			continue
		}
		if err := os.MkdirAll(user.SaveDir, 0o755); err != nil {
			return fmt.Errorf("prepare download directory for user %s: %w", user.Name, err)
		}
	}
	return nil
}

func (r *Runner) enabledUsers() []config.UserConfig {
	var users []config.UserConfig
	for _, user := range r.cfg.Users {
		if user.Enabled {
			users = append(users, user)
		}
	}
	return users
}

func (r *Runner) processUser(ctx context.Context, user config.UserConfig) error {
	r.log.Infof("processing user=%s quality=%s url=%s", user.Name, user.Quality.String(), user.URL)

	videoURLs, err := discovery.NewResolver().ResolveVideoURLs(ctx, user.URL, r.cfg.App.CookiesFile)
	if err != nil {
		errText := sensitive.Redact(err.Error())
		r.log.Errorf("user=%s resolve videos failed: %s", user.Name, errText)
		r.recordFailedDownload(ctx, user, user.URL, errText)
		r.sendNotify(ctx, notify.Event{
			Title:   "抖音视频解析失败",
			User:    user.Name,
			Quality: user.Quality.String(),
			Status:  "failed",
			Error:   errText,
		})
		return fmt.Errorf("process user %s: %s", user.Name, errText)
	}
	r.log.Infof("user=%s resolved %d video url(s)", user.Name, len(videoURLs))

	before, err := archive.ReadIDs(r.cfg.App.ArchiveFile)
	if err != nil {
		return fmt.Errorf("read archive before user %s: %w", user.Name, err)
	}

	attempts := r.cfg.Download.Retries + 1
	var downloadedItems []downloader.DownloadedItem
	var runErr error
	for _, videoURL := range videoURLs {
		result, err := r.downloadVideo(ctx, user, videoURL, attempts)
		if err != nil {
			errText := sensitive.Redact(err.Error())
			r.log.Errorf("user=%s url=%s failed after %d attempts", user.Name, videoURL, attempts)
			r.recordFailedDownload(ctx, user, videoURL, errText)
			r.sendNotify(ctx, notify.Event{
				Title:   "抖音视频下载失败",
				User:    user.Name,
				Quality: user.Quality.String(),
				Status:  "failed",
				Error:   errText,
			})
			runErr = errors.Join(runErr, fmt.Errorf("%s: %s", videoURL, errText))
			continue
		}
		downloadedItems = append(downloadedItems, result.Items...)
	}

	after, err := archive.ReadIDs(r.cfg.App.ArchiveFile)
	if err != nil {
		return fmt.Errorf("read archive after user %s: %w", user.Name, err)
	}
	addedArchiveIDs := archive.Diff(before, after)
	if len(downloadedItems) == 0 {
		downloadedItems = archiveItems(addedArchiveIDs)
	}
	if len(downloadedItems) == 0 {
		r.log.Infof("user=%s no new videos", user.Name)
		return runErr
	}

	for _, item := range downloadedItems {
		record := storage.DownloadRecord{
			UserName: user.Name,
			UserURL:  user.URL,
			VideoID:  item.ID,
			Title:    item.Title,
			FilePath: item.FilePath,
			Quality:  user.Quality.String(),
			Status:   "success",
		}
		if err := r.store.UpsertDownload(ctx, record); err != nil {
			r.log.Warnf("record downloaded video failed: id=%s err=%v", item.ID, err)
			continue
		}
		r.log.Infof("download recorded user=%s id=%s file=%s", user.Name, item.ID, item.FilePath)
		r.sendNotify(ctx, notify.Event{
			Title:    "抖音视频下载完成",
			User:     user.Name,
			Quality:  user.Quality.String(),
			Status:   "success",
			FilePath: item.FilePath,
		})
	}
	if runErr != nil {
		return fmt.Errorf("process user %s: %w", user.Name, runErr)
	}
	return nil
}

func (r *Runner) downloadVideo(ctx context.Context, user config.UserConfig, videoURL string, attempts int) (downloader.Result, error) {
	var result downloader.Result
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(r.cfg.App.TimeoutSeconds)*time.Second)
		job := downloader.JobFromConfig(r.cfg, user)
		job.UserURL = videoURL
		result, lastErr = r.downloader.Run(attemptCtx, job)
		cancel()
		if lastErr == nil {
			break
		}
		r.log.Warnf("user=%s attempt=%d/%d failed: %s", user.Name, attempt, attempts, sensitive.Redact(lastErr.Error()))
	}
	return result, lastErr
}

func (r *Runner) recordFailedDownload(ctx context.Context, user config.UserConfig, targetURL, errText string) {
	record := storage.DownloadRecord{
		UserName: user.Name,
		UserURL:  user.URL,
		VideoID:  storage.FailureID(targetURL, time.Now()),
		Quality:  user.Quality.String(),
		Status:   "failed",
		Error:    errText,
	}
	if err := r.store.UpsertDownload(ctx, record); err != nil {
		r.log.Warnf("record failed download failed: %v", err)
	}
}

func archiveItems(ids []string) []downloader.DownloadedItem {
	items := make([]downloader.DownloadedItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, downloader.DownloadedItem{ID: id})
	}
	return items
}

func (r *Runner) sendNotify(ctx context.Context, event notify.Event) {
	if r.notifier == nil {
		return
	}
	if event.Time == "" {
		event.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	if err := r.notifier.Send(ctx, event); err != nil {
		r.log.Warnf("send notification failed: %v", err)
	}
}
