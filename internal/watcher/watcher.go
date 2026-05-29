package watcher

import (
	"fmt"
	"log"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors a file for changes and triggers a callback
type Watcher struct {
	watcher       *fsnotify.Watcher
	path          string // cleaned absolute/relative path of the watched file
	dir           string // parent directory actually registered with fsnotify
	debounceDelay time.Duration
	onChange      func()
	stopCh        chan struct{}
	errorCount    atomic.Uint64
}

// New creates a new file watcher
func New(path string, debounceDelay time.Duration, onChange func()) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	w := &Watcher{
		watcher:       fsWatcher,
		path:          filepath.Clean(path),
		dir:           filepath.Dir(path),
		debounceDelay: debounceDelay,
		onChange:      onChange,
		stopCh:        make(chan struct{}),
	}

	return w, nil
}

// Start begins watching the file for changes.
//
// We register the parent directory rather than the file itself. Many writers
// (including bd-lite and editors) save atomically by writing a temp file and
// renaming it over the target; an inode-level watch on the file path goes
// silent after such a replace. Watching the directory and filtering events by
// filename survives renames and recreations.
func (w *Watcher) Start() error {
	if err := w.watcher.Add(w.dir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	go w.watchLoop()
	return nil
}

// Stop stops watching the file
func (w *Watcher) Stop() error {
	close(w.stopCh)
	return w.watcher.Close()
}

// ErrorCount returns the number of errors encountered by the watcher
func (w *Watcher) ErrorCount() uint64 {
	return w.errorCount.Load()
}

// watchLoop runs the main watch loop with debouncing
func (w *Watcher) watchLoop() {
	var debounceTimer *time.Timer

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// The directory watch reports events for every file in the dir;
			// ignore anything that is not our target file.
			if filepath.Clean(event.Name) != w.path {
				continue
			}

			// Only respond to write and create events
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				// Debounce: reset timer if it's already running
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(w.debounceDelay, func() {
					w.onChange()
				})
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// Log watcher errors for debugging
			w.errorCount.Add(1)
			log.Printf("WATCHER ERROR: path=%s count=%d error=%v", w.path, w.errorCount.Load(), err)

		case <-w.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return
		}
	}
}
