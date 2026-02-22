// Package watcher monitors a directory for newly created files and emits their
// paths on a channel. It uses fsnotify for native OS events and optionally
// filters by file extension. A generation-based debounce avoids duplicate
// events from rapid write bursts (e.g. large file copies).
package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceDelay = 750 * time.Millisecond

// debounceMsg is sent by a timer goroutine back into the main select loop via
// a dedicated channel, keeping all map operations on a single goroutine.
type debounceMsg struct {
	path string
	gen  int
}

// Watch starts watching dir and sends absolute paths of newly created / written
// files to the returned channel. It stops when stop is closed.
// allowedExts may be nil/empty to allow all extensions.
func Watch(dir string, allowedExts map[string]struct{}, stop <-chan struct{}) (<-chan string, error) {
	out := make(chan string, 16)

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fw.Add(dir); err != nil {
		_ = fw.Close()
		return nil, err
	}

	slog.Info("watching directory", "dir", dir)

	go func() {
		defer close(out)
		defer fw.Close()

		// timerCh is the only channel that timer goroutines write to.
		// All map state is accessed exclusively from within this goroutine.
		timerCh := make(chan debounceMsg, 64)

		timers := make(map[string]*time.Timer) // path → active timer
		gens := make(map[string]int)           // path → current generation

		for {
			select {
			case <-stop:
				return

			case event, ok := <-fw.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
					continue
				}
				path, err := filepath.Abs(event.Name)
				if err != nil {
					continue
				}

				// Cancel any existing timer for this path.
				if t, ok := timers[path]; ok {
					t.Stop()
				}

				// Bump generation; the timer goroutine captures this value.
				gens[path]++
				gen := gens[path]
				p := path

				// Timer goroutine only touches timerCh – safe.
				timers[p] = time.AfterFunc(debounceDelay, func() {
					select {
					case timerCh <- debounceMsg{path: p, gen: gen}:
					case <-stop:
					}
				})

			case msg := <-timerCh:
				// Discard if a newer event has superseded this one.
				if gens[msg.path] != msg.gen {
					continue
				}
				delete(timers, msg.path)
				delete(gens, msg.path)

				if !allowed(msg.path, allowedExts) {
					slog.Debug("skipping file (extension not allowed)", "file", msg.path)
					continue
				}
				if err := waitForFile(msg.path, 2*time.Second); err != nil {
					slog.Warn("file not accessible, skipping", "file", msg.path, "error", err)
					continue
				}
				slog.Info("new file detected, queuing upload", "file", msg.path)
				select {
				case out <- msg.path:
				case <-stop:
					return
				}

			case watchErr, ok := <-fw.Errors:
				if !ok {
					return
				}
				slog.Error("watcher error", "error", watchErr)
			}
		}
	}()

	return out, nil
}

// allowed returns true if the path's extension is in the allowed set,
// or if the allowed set is empty (all extensions permitted).
func allowed(path string, exts map[string]struct{}) bool {
	if len(exts) == 0 {
		return true
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	_, ok := exts[ext]
	return ok
}

// waitForFile blocks until the file at path exists and is readable (or timeout).
func waitForFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_, err := os.Stat(path)
	return err
}
