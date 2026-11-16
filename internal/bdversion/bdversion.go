// Package bdversion decides whether the bd on PATH is one bd-tui supports.
//
// bd-lite reports its version as `bd version <semver>`. Builds before v0.1.0
// reject --version, and untagged builds print a Go pseudo-version, which
// semver orders correctly against the tags around it.
package bdversion

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	// Minimum is the oldest bd that can identify itself.
	Minimum = "v0.1.0"

	// TestedThrough is the newest major.minor bd-tui has been run against.
	// Under 0.x semver a minor release may change the CLI, so the next minor
	// is reported as untested rather than assumed compatible. Raise it after
	// running the integration tests against the new bd.
	TestedThrough = "v0.1"
)

// ErrNotFound is the run error that means bd is not on PATH.
var ErrNotFound = errors.New("bd not found in PATH")

// Status is the verdict on one bd.
type Status int

const (
	// Unchecked is the zero value: nothing was detected, so nothing is shown.
	Unchecked Status = iota
	Compatible
	Missing
	Unknown
	TooOld
	Newer
)

// Result is what Classify found.
type Result struct {
	Status  Status
	Version string
}

// Classify turns the output and error of `bd --version` into a verdict.
func Classify(output string, runErr error) Result {
	if errors.Is(runErr, ErrNotFound) {
		return Result{Status: Missing}
	}
	if runErr != nil {
		return Result{Status: Unknown}
	}
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return Result{Status: Unknown}
	}
	v := fields[len(fields)-1]
	if !semver.IsValid(v) {
		return Result{Status: Unknown}
	}
	switch {
	case semver.Compare(v, Minimum) < 0:
		return Result{Status: TooOld, Version: v}
	case semver.Compare(semver.MajorMinor(v), TestedThrough) > 0:
		return Result{Status: Newer, Version: v}
	default:
		return Result{Status: Compatible, Version: v}
	}
}

// Warning is the status-bar text for r, or "" when there is nothing to say.
func (r Result) Warning() string {
	switch r.Status {
	case Missing:
		return "not found, editing disabled"
	case Unknown:
		return fmt.Sprintf("version unknown, needs %s+", Minimum)
	case TooOld:
		return fmt.Sprintf("%s too old, needs %s+", r.Version, Minimum)
	case Newer:
		return fmt.Sprintf("%s untested, bd-tui knows %s", r.Version, TestedThrough)
	default:
		return ""
	}
}
