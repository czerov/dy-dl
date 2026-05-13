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
	videoURLPattern  = regexp.MustCompile(`https?://(?:www\.)?douyin\.com/video/(\d{10,})`)
	videoPathPattern = regexp.MustCompile(`/video/(\d{10,})`)
	awemeIDPatterns  = []*regexp.Regexp{
		regexp.MustCompile(`(?i)"(?:aweme_id|awemeId|group_id|item_id|video_id)"\s*:\s*"?(\d{10,})"?`),
		regexp.MustCompile(`(?i)(?:aweme_id|awemeId|group_id|item_id|video_id)=["']?(\d{10,})["']?`),
	}
)

type Resolver struct {
	Client    *http.Client
	UserAgent string
}

func NewResolver() *Resolver {
	return &Resolver{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		UserAgent: defaultUserAgent,
	}
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalized, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", r.userAgent())
	req.Header.Set("Referer", "https://www.douyin.com/")
	if cookieHeader, err := CookieHeaderFromFile(cookiesFile); err != nil {
		return nil, err
	} else if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}

	client := r.Client
	if client == nil {
		client = NewResolver().Client
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	finalURL := NormalizeSourceURL(resp.Request.URL.String())
	if id, ok := DouyinVideoID(finalURL); ok {
		return []string{videoURL(id)}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch Douyin page failed: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes))
	if err != nil {
		return nil, err
	}
	ids := ExtractVideoIDs(string(data))
	if len(ids) == 0 {
		return nil, errors.New("no video IDs found on Douyin page; try a direct https://www.douyin.com/video/<id> URL or refresh cookies")
	}

	urls := make([]string, 0, len(ids))
	for _, id := range ids {
		urls = append(urls, videoURL(id))
	}
	return urls, nil
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
		if strings.HasPrefix(parsed.Path, "/user/") || strings.HasPrefix(parsed.Path, "/video/") {
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

func ExtractVideoIDs(page string) []string {
	var ids []string
	seen := map[string]struct{}{}
	add := func(id string) {
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	for _, candidate := range pageVariants(page) {
		for _, match := range videoPathPattern.FindAllStringSubmatch(candidate, -1) {
			if len(match) == 2 {
				add(match[1])
			}
		}
		for _, pattern := range awemeIDPatterns {
			for _, match := range pattern.FindAllStringSubmatch(candidate, -1) {
				if len(match) == 2 {
					add(match[1])
				}
			}
		}
	}
	return ids
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
