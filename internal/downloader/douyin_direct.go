package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const douyinMobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"

var (
	douyinVideoIDPattern = regexp.MustCompile(`https?://(?:www\.)?douyin\.com/video/(\d{10,})`)
	routerDataPattern    = regexp.MustCompile(`(?s)window\._ROUTER_DATA\s*=\s*(\{.*?\})\s*;?\s*</script>`)
)

type douyinVideoMeta struct {
	ID       string
	Title    string
	PlayURI  string
	PlayURLs []string
}

func RunDouyinDirectFallback(ctx context.Context, job Job) (Result, error) {
	id, ok := douyinVideoID(job.UserURL)
	if !ok {
		return Result{}, errors.New("direct fallback only supports Douyin video URLs")
	}
	if archived, err := directArchiveContains(job.ArchiveFile, id); err != nil {
		return Result{}, err
	} else if archived {
		return Result{Output: fmt.Sprintf("direct Douyin fallback skipped archived id=%s", id)}, nil
	}

	client := &http.Client{Timeout: 60 * time.Second}
	cookieHeader, err := directCookieHeaderFromFile(job.CookiesFile)
	if err != nil {
		return Result{}, err
	}
	meta, err := fetchDouyinVideoMeta(ctx, client, id, cookieHeader)
	if err != nil {
		return Result{}, err
	}
	if meta.ID == "" {
		meta.ID = id
	}
	if meta.Title == "" {
		meta.Title = "douyin-" + id
	}

	downloadURL, err := directDouyinDownloadURL(meta, job.Quality)
	if err != nil {
		return Result{}, err
	}
	filePath := directOutputPath(job, meta)
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return Result{}, err
	}
	if err := downloadDirectFile(ctx, client, downloadURL, filePath, cookieHeader); err != nil {
		return Result{}, err
	}
	if err := appendDirectArchive(job.ArchiveFile, meta.ID); err != nil {
		return Result{}, err
	}

	return Result{
		Items: []DownloadedItem{{
			ID:       meta.ID,
			Title:    meta.Title,
			FilePath: filePath,
		}},
		Output: fmt.Sprintf("direct Douyin fallback downloaded id=%s file=%s", meta.ID, filePath),
	}, nil
}

func fetchDouyinVideoMeta(ctx context.Context, client *http.Client, id, cookieHeader string) (douyinVideoMeta, error) {
	pageURL := "https://www.iesdouyin.com/share/video/" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return douyinVideoMeta{}, err
	}
	req.Header.Set("User-Agent", douyinMobileUserAgent)
	req.Header.Set("Referer", "https://www.douyin.com/")
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return douyinVideoMeta{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return douyinVideoMeta{}, fmt.Errorf("fetch Douyin share page failed: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return douyinVideoMeta{}, err
	}
	return parseDouyinVideoMeta(id, string(data))
}

func parseDouyinVideoMeta(id, page string) (douyinVideoMeta, error) {
	page = html.UnescapeString(strings.NewReplacer(`\/`, `/`, `\u002F`, `/`, `\u002f`, `/`).Replace(page))
	match := routerDataPattern.FindStringSubmatch(page)
	if len(match) != 2 {
		return douyinVideoMeta{}, errors.New("Douyin router data not found")
	}
	var payload any
	if err := json.Unmarshal([]byte(match[1]), &payload); err != nil {
		return douyinVideoMeta{}, fmt.Errorf("decode Douyin router data: %w", err)
	}
	item, ok := findDouyinAwemeItem(payload)
	if !ok {
		return douyinVideoMeta{}, errors.New("Douyin video item not found")
	}
	return metaFromDouyinAwemeItem(id, item)
}

func findDouyinAwemeItem(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if list, ok := typed["item_list"].([]any); ok && len(list) > 0 {
			if item, ok := list[0].(map[string]any); ok {
				if _, hasVideo := item["video"]; hasVideo {
					return item, true
				}
			}
		}
		for _, nested := range typed {
			if item, ok := findDouyinAwemeItem(nested); ok {
				return item, true
			}
		}
	case []any:
		for _, nested := range typed {
			if item, ok := findDouyinAwemeItem(nested); ok {
				return item, true
			}
		}
	}
	return nil, false
}

func metaFromDouyinAwemeItem(fallbackID string, item map[string]any) (douyinVideoMeta, error) {
	meta := douyinVideoMeta{
		ID:    firstJSONText(item, "aweme_id", "group_id", "id"),
		Title: firstJSONText(item, "desc", "title"),
	}
	if meta.ID == "" {
		meta.ID = fallbackID
	}
	video, ok := item["video"].(map[string]any)
	if !ok {
		return douyinVideoMeta{}, errors.New("Douyin video field not found")
	}
	playAddr, ok := video["play_addr"].(map[string]any)
	if !ok {
		return douyinVideoMeta{}, errors.New("Douyin play address not found")
	}
	meta.PlayURI = firstJSONText(playAddr, "uri")
	if list, ok := playAddr["url_list"].([]any); ok {
		for _, raw := range list {
			if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
				meta.PlayURLs = append(meta.PlayURLs, strings.TrimSpace(text))
			}
		}
	}
	if meta.PlayURI == "" && len(meta.PlayURLs) == 0 {
		return douyinVideoMeta{}, errors.New("Douyin play URL not found")
	}
	return meta, nil
}

func directDouyinDownloadURL(meta douyinVideoMeta, quality string) (string, error) {
	if meta.PlayURI != "" {
		values := url.Values{}
		values.Set("video_id", meta.PlayURI)
		values.Set("ratio", directDouyinRatio(quality))
		values.Set("line", "0")
		return "https://www.iesdouyin.com/aweme/v1/play/?" + values.Encode(), nil
	}
	if len(meta.PlayURLs) > 0 {
		return meta.PlayURLs[0], nil
	}
	return "", errors.New("Douyin play URL is empty")
}

func directDouyinRatio(quality string) string {
	switch strings.ToLower(strings.TrimSpace(quality)) {
	case "480":
		return "540p"
	case "720":
		return "720p"
	default:
		return "1080p"
	}
}

func downloadDirectFile(ctx context.Context, client *http.Client, rawURL, path, cookieHeader string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", douyinMobileUserAgent)
	req.Header.Set("Referer", "https://www.iesdouyin.com/")
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download direct Douyin video failed: HTTP %d", resp.StatusCode)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}

func directOutputPath(job Job, meta douyinVideoMeta) string {
	userDir := sanitizeFilename(job.UserName)
	if userDir == "" {
		userDir = "douyin"
	}
	title := sanitizeFilename(meta.Title)
	if title == "" {
		title = "douyin-" + meta.ID
	}
	filename := title + "-" + meta.ID + ".mp4"
	return filepath.Join(job.SaveDir, userDir, filename)
}

func sanitizeFilename(value string) string {
	value = strings.TrimSpace(value)
	value = strings.NewReplacer(
		"<", "_",
		">", "_",
		":", "_",
		`"`, "_",
		"/", "_",
		`\`, "_",
		"|", "_",
		"?", "_",
		"*", "_",
		"\r", " ",
		"\n", " ",
		"\t", " ",
	).Replace(value)
	value = strings.Join(strings.Fields(value), " ")
	value = strings.Trim(value, ". ")
	if len([]rune(value)) > 80 {
		value = string([]rune(value)[:80])
	}
	return value
}

func directArchiveContains(path, id string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == id || strings.HasSuffix(line, " "+id) {
			return true, nil
		}
	}
	return false, nil
}

func appendDirectArchive(path, id string) error {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(id) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintln(file, id)
	return err
}

func directCookieHeaderFromFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", nil
	}
	if strings.Contains(content, ";") && !strings.Contains(content, "\t") {
		return directRawCookieHeader(content), nil
	}
	var pairs []string
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		name := strings.TrimSpace(fields[5])
		value := strings.TrimSpace(fields[6])
		if name != "" {
			pairs = append(pairs, name+"="+value)
		}
	}
	return strings.Join(pairs, "; "), nil
}

func directRawCookieHeader(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "cookie:") {
			return strings.TrimSpace(line[len("cookie:"):])
		}
	}
	if strings.HasPrefix(strings.ToLower(content), "cookie:") {
		return strings.TrimSpace(content[len("cookie:"):])
	}
	return content
}

func firstJSONText(data map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := data[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if text := strings.TrimSpace(typed); text != "" {
				return text
			}
		case json.Number:
			if text := strings.TrimSpace(typed.String()); text != "" {
				return text
			}
		case float64:
			if typed > 0 {
				return fmt.Sprintf("%.0f", typed)
			}
		}
	}
	return ""
}

func douyinVideoID(raw string) (string, bool) {
	match := douyinVideoIDPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}
