package bdversion

import (
	"errors"
	"testing"
)

func TestClassify(t *testing.T) {
	for _, tc := range []struct {
		name    string
		output  string
		runErr  error
		want    Status
		version string
	}{
		{"release", "bd version v0.1.0\n", nil, Compatible, "v0.1.0"},
		{"patch release", "bd version v0.1.3\n", nil, Compatible, "v0.1.3"},
		{"pseudo-version after v0.1.0", "bd version v0.1.1-0.20261120101500-abcdef123456\n", nil, Compatible, "v0.1.1-0.20261120101500-abcdef123456"},
		{"dirty build", "bd version v0.1.0+dirty\n", nil, Compatible, "v0.1.0+dirty"},
		{"untagged build before v0.1.0", "bd version v0.0.0-20261113120000-abcdef123456\n", nil, TooOld, "v0.0.0-20261113120000-abcdef123456"},
		{"next minor", "bd version v0.2.0\n", nil, Newer, "v0.2.0"},
		{"next major", "bd version v1.0.0\n", nil, Newer, "v1.0.0"},
		{"predates --version", "", errors.New("exit status 1"), Unknown, ""},
		{"unparseable", "bd version (devel)\n", nil, Unknown, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.output, tc.runErr)
			if got.Status != tc.want || got.Version != tc.version {
				t.Errorf("Classify(%q, %v) = %+v, want status %v version %q", tc.output, tc.runErr, got, tc.want, tc.version)
			}
		})
	}
}

func TestMissing(t *testing.T) {
	if got := Classify("", ErrNotFound); got.Status != Missing {
		t.Errorf("Classify with ErrNotFound = %+v, want Missing", got)
	}
}

// Silent when compatible, and when nothing was checked (tests and the zero
// value); a warning for everything else.
func TestWarning(t *testing.T) {
	for _, tc := range []struct {
		r    Result
		want string
	}{
		{Result{}, ""},
		{Result{Status: Compatible, Version: "v0.1.0"}, ""},
		{Result{Status: Missing}, "not found, editing disabled"},
		{Result{Status: Unknown}, "version unknown, needs v0.1.0+"},
		{Result{Status: TooOld, Version: "v0.0.0-20261113120000-abcdef123456"}, "v0.0.0-20261113120000-abcdef123456 too old, needs v0.1.0+"},
		{Result{Status: Newer, Version: "v0.2.0"}, "v0.2.0 untested, bd-tui knows v0.1"},
	} {
		if got := tc.r.Warning(); got != tc.want {
			t.Errorf("%+v.Warning() = %q, want %q", tc.r, got, tc.want)
		}
	}
}
