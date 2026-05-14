package discovery

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
	maxPageBytes     = 8 << 20
)

var (
	videoURLPattern       = regexp.MustCompile(`https?://(?:www\.)?douyin\.com/video/(\d{10,})`)
	videoPathPattern      = regexp.MustCompile(`/video/(\d{10,})`)
	collectionURLPattern  = regexp.MustCompile(`https?://(?:www\.)?douyin\.com/collection/(\d{10,})`)
	collectionPathPattern = regexp.MustCompile(`/collection/(\d{10,})`)
	seriesURLPattern      = regexp.MustCompile(`https?://(?:www\.)?douyin\.com/series/(\d{10,})`)
	seriesPathPattern     = regexp.MustCompile(`/series/(\d{10,})`)
	awemeIDPatterns       = []*regexp.Regexp{
		regexp.MustCompile(`(?i)"(?:aweme_id|awemeId|group_id|item_id|video_id)"\s*:\s*"?(\d{10,})"?`),
		regexp.MustCompile(`(?i)(?:aweme_id|awemeId|group_id|item_id|video_id)=["']?(\d{10,})["']?`),
	}
	collectionIDPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)"(?:mix_id|mixId|collection_id|collectionId)"\s*:\s*"?(\d{10,})"?`),
		regexp.MustCompile(`(?i)(?:mix_id|mixId|collection_id|collectionId)=["']?(\d{10,})["']?`),
	}
	seriesIDPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)"(?:series_id|seriesId|playlet_id|playletId)"\s*:\s*"?(\d{10,})"?`),
		regexp.MustCompile(`(?i)(?:series_id|seriesId|playlet_id|playletId)=["']?(\d{10,})["']?`),
	}
)

const (
	TypeWork       = "work"
	TypeCollection = "collection"
	TypeSeries     = "series"
)

type Resolver struct {
	Client    *http.Client
	UserAgent string
}

type Result struct {
	SourceURL string      `json:"source_url"`
	Items     []MediaItem `json:"items"`
}

type MediaItem struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
	URL   string `json:"url"`
}

func NewResolver() *Resolver {
	return &Resolver{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		UserAgent: defaultUserAgent,
	}
}

func (r *Resolver) Discover(ctx context.Context, sourceURL, cookiesFile string) (Result, error) {
	normalized := NormalizeSourceURL(sourceURL)
	if normalized == "" {
		return Result{}, errors.New("source URL is empty")
	}
	if id, ok := DouyinVideoID(normalized); ok {
		return Result{
			SourceURL: normalized,
			Items: []MediaItem{{
				ID:   id,
				Type: TypeWork,
				URL:  videoURL(id),
			}},
		}, nil
	}
	if id, ok := DouyinCollectionID(normalized); ok {
		return Result{
			SourceURL: normalized,
			Items: []MediaItem{{
				ID:   id,
				Type: TypeCollection,
				URL:  collectionURL(id),
			}},
		}, nil
	}
	if id, ok := DouyinSeriesID(normalized); ok {
		return Result{
			SourceURL: normalized,
			Items: []MediaItem{{
				ID:   id,
				Type: TypeSeries,
				URL:  seriesURL(id),
			}},
		}, nil
	}

	page, finalURL, err := r.fetchPage(ctx, normalized, cookiesFile)
	if err != nil {
		return Result{}, err
	}
	items := ExtractMediaItems(page)
	if len(items) == 0 {
		return Result{}, errors.New("no works, collections or series found on Douyin page; refresh cookies or try a direct video/collection/series URL")
	}
	return Result{
		SourceURL: finalURL,
		Items:     items,
	}, nil
}

func (r *Resolver) ResolveVideoURLs(ctx context.Context, sourceURL, cookiesFile string) ([]string, error) {
	normalized := NormalizeSourceURL(sourceURL)
	if normalized == "" {
		return nil, errors.New("source URL is empty")
	}
	if id, ok := DouyinVideoID(normalized); ok {
		return []string{videoURL(id)}, nil
	}
	if !isFetchableDouyinURL(normalized) {
		return nil, fmt.Errorf("unsupported URL %q, expected a Douyin user page or video URL", normalized)
	}

	page, finalURL, err := r.fetchPage(ctx, normalized, cookiesFile)
	if err != nil {
		return nil, err
	}
	if id, ok := DouyinVideoID(finalURL); ok {
		return []string{videoURL(id)}, nil
	}
	ids := ExtractVideoIDs(page)
	if len(ids) == 0 {
		return nil, errors.New("no video IDs found on Douyin page; try a direct https://www.douyin.com/video/<id> URL or refresh cookies")
	}

	urls := make([]string, 0, len(ids))
	for _, id := range ids {
		urls = append(urls, videoURL(id))
	}
	return urls, nil
}

func (r *Resolver) fetchPage(ctx context.Context, sourceURL, cookiesFile string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", r.userAgent())
	req.Header.Set("Referer", "https://www.douyin.com/")
	if cookieHeader, err := CookieHeaderFromFile(cookiesFile); err != nil {
		return "", "", err
	} else if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}

	client := r.Client
	if client == nil {
		client = NewResolver().Client
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	finalURL := NormalizeSourceURL(resp.Request.URL.String())
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("fetch Douyin page failed: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes))
	if err != nil {
		return "", "", err
	}
	return string(data), finalURL, nil
}

func (r *Resolver) userAgent() string {
	if strings.TrimSpace(r.UserAgent) != "" {
		return r.UserAgent
	}
	return defaultUserAgent
}

func NormalizeSourceURL(raw string) string {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return value
	}
	if isDouyinHost(parsed.Hostname()) {
		parsed.Fragment = ""
		if strings.HasPrefix(parsed.Path, "/user/") ||
			strings.HasPrefix(parsed.Path, "/video/") ||
			strings.HasPrefix(parsed.Path, "/collection/") ||
			strings.HasPrefix(parsed.Path, "/series/") {
			parsed.RawQuery = ""
		}
	}
	return parsed.String()
}

func DouyinVideoID(raw string) (string, bool) {
	value := NormalizeSourceURL(raw)
	match := videoURLPattern.FindStringSubmatch(value)
	if len(match) == 2 {
		return match[1], true
	}
	return "", false
}

func DouyinCollectionID(raw string) (string, bool) {
	value := NormalizeSourceURL(raw)
	match := collectionURLPattern.FindStringSubmatch(value)
	if len(match) == 2 {
		return match[1], true
	}
	return "", false
}

func DouyinSeriesID(raw string) (string, bool) {
	value := NormalizeSourceURL(raw)
	match := seriesURLPattern.FindStringSubmatch(value)
	if len(match) == 2 {
		return match[1], true
	}
	return "", false
}

func ExtractVideoIDs(page string) []string {
	items := ExtractMediaItems(page)
	var ids []string
	for _, item := range items {
		if item.Type == TypeWork {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

func ExtractMediaItems(page string) []MediaItem {
	var items []MediaItem
	seen := map[string]struct{}{}
	add := func(itemType, id string) {
		if id == "" {
			return
		}
		key := itemType + ":" + id
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		item := MediaItem{
			ID:   id,
			Type: itemType,
		}
		switch itemType {
		case TypeWork:
			item.URL = videoURL(id)
		case TypeCollection:
			item.URL = collectionURL(id)
		case TypeSeries:
			item.URL = seriesURL(id)
		}
		items = append(items, item)
	}

	for _, candidate := range pageVariants(page) {
		for _, match := range videoPathPattern.FindAllStringSubmatch(candidate, -1) {
			if len(match) == 2 {
				add(TypeWork, match[1])
			}
		}
		for _, match := range collectionPathPattern.FindAllStringSubmatch(candidate, -1) {
			if len(match) == 2 {
				add(TypeCollection, match[1])
			}
		}
		for _, match := range seriesPathPattern.FindAllStringSubmatch(candidate, -1) {
			if len(match) == 2 {
				add(TypeSeries, match[1])
			}
		}
		for _, pattern := range awemeIDPatterns {
			for _, match := range pattern.FindAllStringSubmatch(candidate, -1) {
				if len(match) == 2 {
					add(TypeWork, match[1])
				}
			}
		}
		for _, pattern := range collectionIDPatterns {
			for _, match := range pattern.FindAllStringSubmatch(candidate, -1) {
				if len(match) == 2 {
					add(TypeCollection, match[1])
				}
			}
		}
		for _, pattern := range seriesIDPatterns {
			for _, match := range pattern.FindAllStringSubmatch(candidate, -1) {
				if len(match) == 2 {
					add(TypeSeries, match[1])
				}
			}
		}
	}
	return items
}

func CookieHeaderFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", nil
	}
	if strings.Contains(content, ";") && !strings.Contains(content, "\t") {
		return rawCookieHeader(content), nil
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
		if name == "" {
			continue
		}
		pairs = append(pairs, name+"="+value)
	}
	return strings.Join(pairs, "; "), nil
}

func pageVariants(page string) []string {
	normalized := strings.NewReplacer(`\/`, `/`, `\u002F`, `/`, `\u002f`, `/`).Replace(page)
	variants := []string{normalized}
	unescapedHTML := html.UnescapeString(normalized)
	if unescapedHTML != normalized {
		variants = append(variants, unescapedHTML)
	}
	if decoded, err := url.QueryUnescape(unescapedHTML); err == nil && decoded != unescapedHTML {
		variants = append(variants, strings.NewReplacer(`\/`, `/`, `\u002F`, `/`, `\u002f`, `/`).Replace(decoded))
	}
	return variants
}

func rawCookieHeader(content string) string {
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

func isFetchableDouyinURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && isDouyinHost(parsed.Hostname())
}

func isDouyinHost(host string) bool {
	host = strings.ToLower(host)
	return host == "douyin.com" || strings.HasSuffix(host, ".douyin.com") ||
		host == "iesdouyin.com" || strings.HasSuffix(host, ".iesdouyin.com")
}

func videoURL(id string) string {
	return "https://www.douyin.com/video/" + id
}

func collectionURL(id string) string {
	return "https://www.douyin.com/collection/" + id
}

func seriesURL(id string) string {
	return "https://www.douyin.com/series/" + id
}
