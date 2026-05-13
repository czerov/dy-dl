package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"douyin-nas-monitor/internal/config"
	"douyin-nas-monitor/internal/downloader"
	"douyin-nas-monitor/internal/logger"
	"douyin-nas-monitor/internal/monitor"
	"douyin-nas-monitor/internal/notify"
	"douyin-nas-monitor/internal/storage"
)

const version = "0.1.0"

func main() {
	var (
		configPath  string
		onceFlag    bool
		daemonFlag  bool
		checkFlag   bool
		versionFlag bool
	)

	flag.StringVar(&configPath, "config", "config.yaml", "path to config file")
	flag.BoolVar(&onceFlag, "once", false, "run once and exit")
	flag.BoolVar(&daemonFlag, "daemon", false, "run forever using app.interval_minutes")
	flag.BoolVar(&checkFlag, "check", false, "check configuration and runtime dependencies")
	flag.BoolVar(&versionFlag, "version", false, "print version")
	flag.Parse()

	if versionFlag {
		fmt.Printf("douyin-nas-monitor %s\n", version)
		return
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	if onceFlag {
		cfg.App.Mode = config.ModeOnce
	}
	if daemonFlag {
		cfg.App.Mode = config.ModeDaemon
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if checkFlag {
		results := monitor.CheckEnvironment(ctx, cfg)
		failed := false
		for _, result := range results {
			if result.OK {
				fmt.Printf("[OK] %s\n", result.Name)
				continue
			}
			failed = true
			fmt.Printf("[FAIL] %s: %s\n", result.Name, result.Message)
		}
		if failed {
			os.Exit(1)
		}
		return
	}

	log, err := logger.New(cfg.App.LogFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger failed: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	store, err := storage.Open(cfg.App.Database)
	if err != nil {
		log.Errorf("init database failed: %v", err)
		os.Exit(1)
	}
	defer store.Close()

	runner := monitor.NewRunner(cfg, log, store, downloader.New(), notify.NewGeneric(cfg.Notify))
	if cfg.App.Mode == config.ModeDaemon {
		err = runner.RunDaemon(ctx)
	} else {
		err = runner.RunOnce(ctx)
	}
	if err != nil {
		log.Errorf("run failed: %v", err)
		os.Exit(1)
	}
}
