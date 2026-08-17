package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/andynu/bd-lite-tui/internal/config"
	"github.com/andynu/bd-lite-tui/internal/storage"
	"github.com/andynu/bd-lite-tui/internal/theme"
	_ "github.com/andynu/bd-lite-tui/internal/theme" // Import to register themes
)

func main() {
	// Parse command line flags
	debugMode := flag.Bool("debug", false, "Enable debug logging to file")
	themeName := flag.String("theme", "", "Color theme (default, gruvbox-dark, etc)")
	viewMode := flag.String("view", "list", "Initial view mode (list or tree)")
	issueID := flag.String("issue", "", "Show only this issue (e.g., tui-abc)")
	flag.Parse()

	// Load user config (includes theme preference)
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v, using defaults\n", err)
		cfg = config.DefaultConfig()
	}

	// Theme priority order: CLI flag > env var > config file > default
	// Start with theme from config file
	if cfg.Theme != "" {
		if err := theme.SetCurrent(cfg.Theme); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v, using gruvbox-dark theme\n", err)
			_ = theme.SetCurrent("gruvbox-dark")
		}
	} else {
		_ = theme.SetCurrent("gruvbox-dark")
	}

	// Override with environment variable if set
	if envTheme := os.Getenv("BEADS_THEME"); envTheme != "" && *themeName == "" {
		if err := theme.SetCurrent(envTheme); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v, keeping current theme\n", err)
		}
	}

	// Override with CLI flag if specified (highest priority)
	if *themeName != "" {
		if err := theme.SetCurrent(*themeName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v, keeping current theme\n", err)
		}
	}

	// Set up logging
	var logFile *os.File
	if *debugMode {
		logDir := filepath.Join(os.Getenv("HOME"), ".bd-tui")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create log directory: %v\n", err)
		} else {
			logPath := filepath.Join(logDir, fmt.Sprintf("debug-%s.log", time.Now().Format("2006-01-02-15-04-05")))
			var err error
			logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to open log file: %v\n", err)
			} else {
				log.SetOutput(logFile)
				log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
				defer logFile.Close()
				log.Printf("=== bd-tui started in debug mode ===")
				log.Printf("Log file: %s", logPath)
				fmt.Fprintf(os.Stderr, "Debug logging enabled: %s\n", logPath)
			}
		}
	} else {
		// Disable logging completely when not in debug mode
		log.SetOutput(io.Discard)
		log.SetFlags(0)
	}

	log.Printf("Finding .beads directory")
	// Find .beads directory
	beadsDir, err := storage.FindBeadsDir()
	if err != nil {
		log.Printf("ERROR: Failed to find .beads directory: %v", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	log.Printf("Found .beads directory: %s", beadsDir)

	// Warn if bd CLI is not available (issue updates won't work)
	if _, err := exec.LookPath("bd"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: 'bd' command not found in PATH. Issue updates will not work.\n")
		fmt.Fprintf(os.Stderr, "Install bd-lite or add 'bd' to your PATH to enable editing.\n\n")
	}

	// A missing issues.jsonl is not a failure. bd-lite treats a .beads without
	// one as an empty store, and so does the TUI: it opens an empty board with
	// the create hint in the list, and the watcher (which watches the .beads
	// directory, not the file inode) picks the file up when bd first writes it.
	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")

	tuiApp, err := newTUI(tuiOptions{
		BeadsDir:  beadsDir,
		JSONLPath: jsonlPath,
		IssueID:   *issueID,
		TreeView:  *viewMode == "tree",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer tuiApp.Close()

	// Set up signal handler for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Use sync.Once to ensure shutdown happens exactly once
	var shutdownOnce sync.Once

	// Run signal handler in goroutine
	go func() {
		for sig := range sigChan {
			shutdownOnce.Do(func() {
				log.Printf("SIGNAL: Received signal %v, initiating graceful shutdown", sig)
				tuiApp.Shutdown()
				log.Printf("SIGNAL: Graceful shutdown complete")
			})

			// Log duplicate shutdown attempts
			log.Printf("SIGNAL: Received additional signal %v, shutdown already in progress", sig)
		}
	}()

	if err := tuiApp.Run(); err != nil {
		panic(err)
	}
}

// The TUI itself (widgets, key bindings, refresh plumbing) lives in tui.go so
// it can be constructed against a tcell SimulationScreen from tests.
