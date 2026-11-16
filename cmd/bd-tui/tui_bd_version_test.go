package main

import (
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/bdversion"
)

func withBdVersion(r bdversion.Result) harnessOption {
	return func(c *harnessConfig) { c.tui.BdVersion = r }
}

// The warning is drawn, not just composed: tview reads "[word]" as a style tag
// and would swallow a bare bracketed field.
func TestBdVersion_WarningReachesTheScreen(t *testing.T) {
	h := newHarness(t, basicIssues(), withBdVersion(bdversion.Result{Status: bdversion.Newer, Version: "v0.2.0"}))
	if screen := h.screenText(); !strings.Contains(screen, "[bd: v0.2.0 untested, bd-tui knows v0.1]") {
		t.Errorf("bd warning not on screen:\n%s", screen)
	}
}

func TestBdVersion_CompatibleBdSaysNothing(t *testing.T) {
	h := newHarness(t, basicIssues(), withBdVersion(bdversion.Result{Status: bdversion.Compatible, Version: "v0.1.0"}))
	if screen := h.screenText(); strings.Contains(screen, "[bd:") {
		t.Errorf("compatible bd produced a warning:\n%s", screen)
	}
}
