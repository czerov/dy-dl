package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
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
	collectionURLPattern  = regexp.MustCompile(`https?://(?:www\.)?douyin\.com/(?:collection|mix/detail)/(\d{10,})`)
	collectionPathPattern = regexp.MustCompile(`/(?:collection|mix/detail)/(\d{10,})`)
	seriesURLPattern      = regexp.MustCompile(`https?://(?:www\.)?douyin\.com/(?:series|playlet)/(\d{10,})`)
	seriesPathPattern     = regexp.MustCompile(`/(?:series|playlet)/(\d{10,})`)
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

	var items []MediaItem
	var finalURL string
	var lastErr error
	for _, candidateURL := range discoveryCandidateURLs(normalized) {
		page, fetchedURL, err := r.fetchPage(ctx, candidateURL, cookiesFile)
		if err != nil {
			lastErr = err
			continue
		}
		if finalURL == "" {
			finalURL = fetchedURL
		}
		items = mergeMediaItems(items, ExtractMediaItems(page))
	}
	if len(items) == 0 && lastErr != nil {
		return Result{}, lastErr
	}
	if len(items) == 0 {
		return Result{}, errors.New("no works, collections or series found on Douyin page; refresh cookies or try a direct video/collection/series URL")
	}
	return Result{
		SourceURL: finalURL,
		Items:     items,
	}, nil
}

func ImportMediaItems(sourceURL, content string) (Result, error) {
	sourceURL = NormalizeSourceURL(sourceURL)
	content = strings.TrimSpace(content)
	if content == "" {
		return Result{}, errors.New("import content is empty")
	}

	items := parseImportedJSONMediaItems(content)
	if len(items) == 0 {
		items = ExtractMediaItems(content)
	}
	items = mergeMediaItems(nil, items)
	if len(items) == 0 {
		return Result{}, errors.New("no Douyin works, collections or series found in imported content")
	}
	return Result{
		SourceURL: sourceURL,
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

func discoveryCandidateURLs(raw string) []string {
	normalized := NormalizeSourceURL(raw)
	parsed, err := url.Parse(normalized)
	if err != nil || !isDouyinHost(parsed.Hostname()) || !strings.HasPrefix(parsed.Path, "/user/") {
		return []string{normalized}
	}

	base := *parsed
	base.RawQuery = ""
	base.Fragment = ""
	candidates := []string{base.String()}
	for _, tab := range []string{
		"post",
		"collection",
		"series",
		"playlet",
		"short_drama",
		"shortDrama",
	} {
		next := base
		query := next.Query()
		query.Set("showTab", tab)
		next.RawQuery = query.Encode()
		candidates = append(candidates, next.String())
	}
	return candidates
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

func mergeMediaItems(base, next []MediaItem) []MediaItem {
	if len(base) == 0 {
		base = []MediaItem{}
	}
	seen := make(map[string]int, len(base)+len(next))
	for index, item := range base {
		seen[item.Type+":"+item.ID] = index
	}
	for _, item := range next {
		key := item.Type + ":" + item.ID
		if index, ok := seen[key]; ok {
			if base[index].Title == "" && item.Title != "" {
				base[index].Title = item.Title
			}
			if base[index].URL == "" && item.URL != "" {
				base[index].URL = item.URL
			}
			continue
		}
		seen[key] = len(base)
		base = append(base, item)
	}
	return base
}

func parseImportedJSONMediaItems(content string) []MediaItem {
	decoder := json.NewDecoder(bytes.NewReader([]byte(content)))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return nil
	}
	var items []MediaItem
	collectImportedJSONMediaItems(payload, &items)
	return items
}

func collectImportedJSONMediaItems(value any, items *[]MediaItem) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectImportedJSONMediaItems(item, items)
		}
	case map[string]any:
		if item, ok := mediaItemFromImportMap(typed); ok {
			*items = append(*items, item)
			return
		}
		for _, key := range []string{"items", "data", "list", "aweme_list", "awemeList", "videos"} {
			if nested, ok := typed[key]; ok {
				collectImportedJSONMediaItems(nested, items)
			}
		}
	}
}

func mediaItemFromImportMap(data map[string]any) (MediaItem, bool) {
	rawURL := normalizeImportedURL(firstImportString(data, "url", "href", "link", "share_url", "shareUrl"))
	title := firstImportString(data, "title", "desc", "description", "name", "text")
	if rawURL != "" {
		if id, ok := DouyinVideoID(rawURL); ok {
			return MediaItem{ID: id, Type: TypeWork, Title: title, URL: videoURL(id)}, true
		}
		if id, ok := DouyinCollectionID(rawURL); ok {
			return MediaItem{ID: id, Type: TypeCollection, Title: title, URL: collectionURL(id)}, true
		}
		if id, ok := DouyinSeriesID(rawURL); ok {
			return MediaItem{ID: id, Type: TypeSeries, Title: title, URL: seriesURL(id)}, true
		}
	}

	itemType := normalizeImportedType(firstImportString(data, "type", "kind", "category", "tab"))
	id := firstImportString(
		data,
		"id",
		"aweme_id",
		"awemeId",
		"group_id",
		"item_id",
		"video_id",
		"mix_id",
		"mixId",
		"collection_id",
		"collectionId",
		"series_id",
		"seriesId",
		"playlet_id",
		"playletId",
	)
	if itemType == "" || !looksLikeDouyinID(id) {
		return MediaItem{}, false
	}

	item := MediaItem{
		ID:    id,
		Type:  itemType,
		Title: title,
	}
	switch itemType {
	case TypeWork:
		item.URL = videoURL(id)
	case TypeCollection:
		item.URL = collectionURL(id)
	case TypeSeries:
		item.URL = seriesURL(id)
	default:
		return MediaItem{}, false
	}
	return item, true
}

func firstImportString(data map[string]any, keys ...string) string {
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
				return strconv.FormatInt(int64(typed), 10)
			}
		}
	}
	return ""
}

func normalizeImportedType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case TypeWork, "video", "aweme", "post", "作品":
		return TypeWork
	case TypeCollection, "mix", "合集":
		return TypeCollection
	case TypeSeries, "playlet", "short_drama", "shortdrama", "drama", "短剧":
		return TypeSeries
	default:
		return ""
	}
}

func normalizeImportedURL(raw string) string {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "//") {
		return NormalizeSourceURL("https:" + value)
	}
	if strings.HasPrefix(value, "/") {
		return NormalizeSourceURL("https://www.douyin.com" + value)
	}
	return NormalizeSourceURL(value)
}

func looksLikeDouyinID(value string) bool {
	if len(value) < 10 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
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
