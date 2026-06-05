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
	dir           string // parent directory of the file, registered with fsnotify
	parentDir     string // parent of dir, watched to detect dir removal/recreation
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

	dir := filepath.Dir(path)
	w := &Watcher{
		watcher:       fsWatcher,
		path:          filepath.Clean(path),
		dir:           filepath.Clean(dir),
		parentDir:     filepath.Clean(filepath.Dir(dir)),
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
// filename survives renames and recreations of the file.
//
// We additionally watch the grandparent directory. When the whole .beads
// directory is removed and recreated (git checkout/stash/rebase, worktree
// swaps, `bd init`), the kernel silently drops the inotify watch on it and no
// error is reported — live updates would stall permanently. Watching the parent
// lets us see the directory reappear and re-establish the watch.
func (w *Watcher) Start() error {
	if err := w.watcher.Add(w.dir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Best-effort: dir-recreation recovery. If the parent can't be watched we
	// still handle the common in-place/rename cases correctly.
	if w.parentDir != "" && w.parentDir != w.dir {
		if err := w.watcher.Add(w.parentDir); err != nil {
			log.Printf("WATCHER: could not watch parent dir %s (dir-recreation recovery disabled): %v", w.parentDir, err)
		}
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

// rewatchDir re-establishes the watch on the target directory after it has been
// recreated. The kernel drops the original watch on directory deletion, so this
// must be called once the directory exists again.
func (w *Watcher) rewatchDir() {
	if err := w.watcher.Add(w.dir); err != nil {
		w.errorCount.Add(1)
		log.Printf("WATCHER: failed to re-add watch on %s after recreation: %v", w.dir, err)
		return
	}
	log.Printf("WATCHER: re-established watch on %s after recreation", w.dir)
}

// watchLoop runs the main watch loop with debouncing
func (w *Watcher) watchLoop() {
	var debounceTimer *time.Timer

	// trigger (re)starts the debounce timer; the latest change always wins, so a
	// burst of writes collapses into a single onChange after the file settles.
	trigger := func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(w.debounceDelay, w.onChange)
	}

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			cleaned := filepath.Clean(event.Name)

			// The target directory itself was created/removed/renamed (reported
			// by the parent-dir watch). When it reappears, re-establish the watch
			// and reload — writes that happened before the watch was restored are
			// otherwise lost.
			if cleaned == w.dir {
				if event.Op&fsnotify.Create == fsnotify.Create {
					w.rewatchDir()
					trigger()
				}
				continue
			}

			// The directory watch reports events for every file in the dir;
			// ignore anything that is not our target file.
			if cleaned != w.path {
				continue
			}

			// Only respond to write and create events. Atomic renames over the
			// target surface as Create.
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				trigger()
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
