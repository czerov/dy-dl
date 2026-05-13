package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"douyin-nas-monitor/internal/config"
)

type Event struct {
	Title    string `json:"title"`
	User     string `json:"user"`
	Quality  string `json:"quality"`
	Status   string `json:"status"`
	FilePath string `json:"file_path,omitempty"`
	Error    string `json:"error,omitempty"`
	Time     string `json:"time"`
}

type Notifier interface {
	Send(ctx context.Context, event Event) error
}

type Generic struct {
	enabled    bool
	webhookURL string
	client     *http.Client
}

func NewGeneric(cfg config.NotifyConfig) *Generic {
	return &Generic{
		enabled:    cfg.Enabled,
		webhookURL: cfg.WebhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (g *Generic) Send(ctx context.Context, event Event) error {
	if g == nil || !g.enabled || g.webhookURL == "" {
		return nil
	}
	if event.Time == "" {
		event.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPStatusError{StatusCode: resp.StatusCode}
	}
	return nil
}

type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return http.StatusText(e.StatusCode)
}
