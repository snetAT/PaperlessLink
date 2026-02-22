// Package logger initialises a structured slog logger that writes to stdout
// and, optionally, to an additional log file simultaneously.
package logger

import (
	"io"
	"log/slog"
	"os"
)

// Init sets the default slog logger. When logFile is empty only stdout is used.
// Returns a cleanup function that closes the log file (if any).
func Init(logFile string) (cleanup func(), err error) {
	writers := []io.Writer{os.Stdout}

	var f *os.File
	if logFile != "" {
		f, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		writers = append(writers, f)
	}

	mw := io.MultiWriter(writers...)
	handler := slog.NewJSONHandler(mw, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))

	cleanup = func() {
		if f != nil {
			_ = f.Close()
		}
	}
	return cleanup, nil
}
