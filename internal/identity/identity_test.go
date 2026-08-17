package identity

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHuman(t *testing.T) {
	for _, tc := range []struct {
		name     string
		assignee string
		want     string
	}{
		{"plain name is already human-level", "andy", "andy"},
		{"agent tag is stripped", "andy/claude-xyz", "andy"},
		{"only the first segment counts", "andy/claude/nested", "andy"},
		{"surrounding space is trimmed", "  andy  ", "andy"},
		{"space around the slash is trimmed", "andy / claude-xyz", "andy"},
		{"a full name survives intact", "Andy Nutter-Upham", "Andy Nutter-Upham"},
		{"empty stays empty", "", ""},
		{"a bare agent tag has no human", "/claude-xyz", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Human(tc.assignee); got != tc.want {
				t.Errorf("Human(%q) = %q, want %q", tc.assignee, got, tc.want)
			}
		})
	}
}

func TestIsMine(t *testing.T) {
	for _, tc := range []struct {
		name     string
		assignee string
		me       string
		want     bool
	}{
		{"exact match", "andy", "andy", true},
		{"agent tag counts as its human", "andy/claude-xyz", "andy", true},
		{"case is ignored", "Andy", "andy", true},
		{"case is ignored under a tag", "ANDY/claude-xyz", "andy", true},
		{"a full name matches itself", "Andy Nutter-Upham", "Andy Nutter-Upham", true},
		{"a full name matches under a tag", "Andy Nutter-Upham/claude-xyz", "Andy Nutter-Upham", true},
		{"someone else is not mine", "scott", "andy", false},
		{"someone else's agent is not mine", "scott/claude-xyz", "andy", false},
		{"a prefix of my name is not me", "and", "andy", false},
		{"unassigned is nobody's", "", "andy", false},
		{"an unknown identity matches nothing", "andy", "", false},
		{"unknown identity does not match unassigned", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsMine(tc.assignee, tc.me); got != tc.want {
				t.Errorf("IsMine(%q, %q) = %v, want %v", tc.assignee, tc.me, got, tc.want)
			}
		})
	}
}

// Resolve reads user.name out of the repository it is pointed at, not the
// developer's global config.
func TestResolve_ReadsRepoConfig(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	gitIn(t, dir, "init")
	gitIn(t, dir, "config", "user.name", "Repo Local Name")

	if got := Resolve(dir); got != "Repo Local Name" {
		t.Errorf("Resolve(%q) = %q, want %q", dir, got, "Repo Local Name")
	}
}

// An unset user.name yields "", the signal callers use to disable auto-claim
// and "mine" filtering. HOME and the system config are redirected so a global
// user.name on the machine running the test cannot leak in.
func TestResolve_UnsetYieldsEmpty(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig-absent"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(home, "gitconfig-absent"))
	gitIn(t, dir, "init")

	if got := Resolve(dir); got != "" {
		t.Errorf("Resolve with no user.name = %q, want %q", got, "")
	}
}

// Pointing Resolve somewhere that is not a repository is not an error, it is
// simply an unknown identity.
func TestResolve_OutsideARepository(t *testing.T) {
	requireGit(t)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig-absent"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(home, "gitconfig-absent"))

	if got := Resolve(t.TempDir()); got != "" {
		t.Errorf("Resolve outside a repo = %q, want %q", got, "")
	}
}

// A directory that does not exist yields "" rather than a panic or a hang.
func TestResolve_MissingDirectory(t *testing.T) {
	requireGit(t)

	if got := Resolve(filepath.Join(t.TempDir(), "does-not-exist")); got != "" {
		t.Errorf("Resolve of a missing dir = %q, want %q", got, "")
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
