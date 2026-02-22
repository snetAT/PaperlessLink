package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"paperlesslink/config"
	"paperlesslink/logger"
	"paperlesslink/uploader"
	"paperlesslink/watcher"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	var (
		dir          = flag.String("dir", "", "Directory to watch for new files (required)")
		paperlessURL = flag.String("url", "", "Paperless-ngx base URL, e.g. https://paperless.example.com (required)")
		token        = flag.String("token", "", "Paperless-ngx API token (required)")
		ext          = flag.String("ext", "", "Comma-separated allowed file extensions, e.g. pdf,png (empty = all)")
		renameUUID   = flag.Bool("rename-uuid", false, "Rename file to UUID before upload (original name used as title)")
		afterUpload  = flag.String("after-upload", "delete", "Action after upload: delete | backup")
		backupDir    = flag.String("backup-dir", "", "Backup directory (required when -after-upload=backup)")
		logFile      = flag.String("log-file", "", "Path to log file (default: stdout only)")
		pollInterval = flag.Duration("poll-interval", 5*time.Second, "Fallback poll interval for fsnotify")
		showVersion  = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("paperlesslink", version)
		os.Exit(0)
	}

	// Initialise logger first so all subsequent messages are structured.
	cleanup, err := logger.Init(*logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	slog.Info("PaperlessLink starting", "version", version)

	cfg := &config.Config{
		WatchDir:     *dir,
		PaperlessURL: *paperlessURL,
		Token:        *token,
		AllowedExts:  config.ParseExtensions(*ext),
		RenameToUUID: *renameUUID,
		AfterUpload:  config.AfterUpload(*afterUpload),
		BackupDir:    *backupDir,
		LogFile:      *logFile,
		PollInterval: *pollInterval,
	}

	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		flag.Usage()
		os.Exit(2)
	}

	// Ensure backup directory exists when needed.
	if cfg.AfterUpload == config.AfterUploadBackup {
		if err := os.MkdirAll(cfg.BackupDir, 0o755); err != nil {
			slog.Error("cannot create backup dir", "dir", cfg.BackupDir, "error", err)
			os.Exit(1)
		}
	}

	stop := make(chan struct{})

	files, err := watcher.Watch(cfg.WatchDir, cfg.AllowedExts, stop)
	if err != nil {
		slog.Error("failed to start watcher", "error", err)
		os.Exit(1)
	}

	// Handle OS signals for graceful shutdown.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		slog.Info("received signal, shutting down", "signal", sig)
		close(stop)
	}()

	slog.Info("watching for files",
		"dir", cfg.WatchDir,
		"extensions", *ext,
		"after_upload", cfg.AfterUpload,
		"rename_uuid", cfg.RenameToUUID,
	)

	// Main upload loop.
	for filePath := range files {
		if err := uploader.Upload(cfg, filePath); err != nil {
			slog.Error("upload error", "file", filePath, "error", err)
		}
	}

	slog.Info("PaperlessLink stopped")
}
