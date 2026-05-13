package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"douyin-nas-monitor/internal/config"
	"douyin-nas-monitor/internal/downloader"
	"douyin-nas-monitor/internal/logger"
	"douyin-nas-monitor/internal/monitor"
	"douyin-nas-monitor/internal/notify"
	"douyin-nas-monitor/internal/storage"
)

//go:embed static/*
var embeddedStatic embed.FS

type Server struct {
	configPath string
	version    string
	log        *logger.Logger
	store      *storage.Store

	mu             sync.Mutex
	cfg            config.Config
	running        bool
	lastStartedAt  time.Time
	lastFinishedAt time.Time
	lastRunError   string
}

type configResponse struct {
	Config       config.Config `json:"config"`
	ConfigPath   string        `json:"config_path"`
	CookiesExist bool          `json:"cookies_exist"`
}

type updateConfigRequest struct {
	Config config.Config `json:"config"`
}

type statusResponse struct {
	Version        string `json:"version"`
	ConfigPath     string `json:"config_path"`
	Mode           string `json:"mode"`
	Running        bool   `json:"running"`
	UsersTotal     int    `json:"users_total"`
	UsersEnabled   int    `json:"users_enabled"`
	LastStartedAt  string `json:"last_started_at,omitempty"`
	LastFinishedAt string `json:"last_finished_at,omitempty"`
	LastRunError   string `json:"last_run_error,omitempty"`
}

func New(configPath string, cfg config.Config, log *logger.Logger, store *storage.Store, version string) *Server {
	return &Server{
		configPath: configPath,
		cfg:        cfg,
		log:        log,
		store:      store,
		version:    version,
	}
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	handler, err := s.routes()
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	s.log.Infof("web server listening on %s", addr)
	err = httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) routes() (http.Handler, error) {
	staticFS, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/run", s.handleRun)
	mux.HandleFunc("/api/check", s.handleCheck)
	mux.HandleFunc("/api/downloads", s.handleDownloads)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.Handle("/", spaHandler(staticFS))
	return mux, nil
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	cfg, running, started, finished, runErr := s.snapshot()
	resp := statusResponse{
		Version:      s.version,
		ConfigPath:   s.configPath,
		Mode:         cfg.App.Mode,
		Running:      running,
		UsersTotal:   len(cfg.Users),
		UsersEnabled: enabledUserCount(cfg.Users),
		LastRunError: runErr,
	}
	if !started.IsZero() {
		resp.LastStartedAt = started.Format(time.RFC3339)
	}
	if !finished.IsZero() {
		resp.LastFinishedAt = finished.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, _, _, _, _ := s.snapshot()
		writeJSON(w, http.StatusOK, configResponse{
			Config:       cfg,
			ConfigPath:   s.configPath,
			CookiesExist: fileExists(cfg.App.CookiesFile),
		})
	case http.MethodPut:
		var req updateConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := req.Config.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := config.Save(s.configPath, req.Config); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		normalized := req.Config
		if abs, err := filepath.Abs(s.configPath); err == nil {
			normalized = normalized.WithRelativePaths(filepath.Dir(abs))
		}
		s.mu.Lock()
		s.cfg = normalized
		s.mu.Unlock()

		writeJSON(w, http.StatusOK, configResponse{
			Config:       normalized,
			ConfigPath:   s.configPath,
			CookiesExist: fileExists(normalized.App.CookiesFile),
		})
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, errors.New("download task is already running"))
		return
	}
	cfg := s.cfg
	s.running = true
	s.lastStartedAt = time.Now()
	s.lastFinishedAt = time.Time{}
	s.lastRunError = ""
	s.mu.Unlock()

	go s.runOnce(cfg)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "started",
	})
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	cfg, _, _, _, _ := s.snapshot()
	writeJSON(w, http.StatusOK, monitor.CheckEnvironment(r.Context(), cfg))
}

func (s *Server) handleDownloads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.store.ListDownloads(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	lines, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	if lines <= 0 || lines > 1000 {
		lines = 200
	}
	cfg, _, _, _, _ := s.snapshot()
	text, err := tailFile(cfg.App.LogFile, lines)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]string{"text": ""})
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"text": text})
}

func (s *Server) runOnce(cfg config.Config) {
	runner := monitor.NewRunner(cfg, s.log, s.store, downloader.New(), notify.NewGeneric(cfg.Notify))
	err := runner.RunOnce(context.Background())

	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	s.lastFinishedAt = time.Now()
	if err != nil {
		s.lastRunError = err.Error()
		return
	}
	s.lastRunError = ""
}

func (s *Server) snapshot() (config.Config, bool, time.Time, time.Time, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg, s.running, s.lastStartedAt, s.lastFinishedAt, s.lastRunError
}

func enabledUserCount(users []config.UserConfig) int {
	count := 0
	for _, user := range users {
		if user.Enabled {
			count++
		}
	}
	return count
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func tailFile(path string, lines int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	parts := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n"), nil
}

func spaHandler(staticFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(staticFS, path); err != nil {
			r.URL.Path = "/"
			http.ServeFileFS(w, r, staticFS, "index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{
		"error": err.Error(),
	})
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}
