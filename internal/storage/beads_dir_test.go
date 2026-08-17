package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindBeadsDir_InCurrentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads: %v", err)
	}

	t.Chdir(tmpDir)

	got, err := FindBeadsDir()
	if err != nil {
		t.Fatalf("FindBeadsDir failed: %v", err)
	}
	assertSameDir(t, got, beadsDir)
}

func TestFindBeadsDir_WalksUpToParent(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads: %v", err)
	}
	nested := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("failed to create nested dirs: %v", err)
	}

	t.Chdir(nested)

	got, err := FindBeadsDir()
	if err != nil {
		t.Fatalf("FindBeadsDir failed: %v", err)
	}
	assertSameDir(t, got, beadsDir)
}

// A plain file named .beads is not a store, so the walk must keep going.
func TestFindBeadsDir_IgnoresNonDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads: %v", err)
	}
	child := filepath.Join(tmpDir, "child")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatalf("failed to create child dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(child, ".beads"), []byte("not a dir"), 0644); err != nil {
		t.Fatalf("failed to create .beads file: %v", err)
	}

	t.Chdir(child)

	got, err := FindBeadsDir()
	if err != nil {
		t.Fatalf("FindBeadsDir failed: %v", err)
	}
	assertSameDir(t, got, beadsDir)
}

func TestFindBeadsDir_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	skipIfAncestorHasBeads(t, tmpDir)

	t.Chdir(tmpDir)

	if _, err := FindBeadsDir(); err == nil {
		t.Fatal("Expected error when no .beads directory exists")
	}
}

// assertSameDir compares paths after EvalSymlinks, since os.Getwd resolves the
// /tmp -> /private/tmp style symlinks that t.TempDir hands back.
func assertSameDir(t *testing.T, got, want string) {
	t.Helper()
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", got, err)
	}
	wantResolved, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", want, err)
	}
	if gotResolved != wantResolved {
		t.Errorf("Expected %q, got %q", wantResolved, gotResolved)
	}
}

// skipIfAncestorHasBeads keeps the not-found case honest on a machine where the
// temp dir happens to sit under a real beads repo.
func skipIfAncestorHasBeads(t *testing.T, dir string) {
	t.Helper()
	for {
		if info, err := os.Stat(filepath.Join(dir, ".beads")); err == nil && info.IsDir() {
			t.Skipf("ancestor %q has a .beads directory", dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}
