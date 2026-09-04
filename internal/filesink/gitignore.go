package filesink

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

// CheckPath refuses a path that sits inside a git worktree and is not ignored.
// It returns ErrNotIgnored in that case and nil when the path is ignored, is not
// inside a worktree, or git is not installed. The target file may not exist yet,
// so git is run from the nearest existing ancestor directory.
func CheckPath(path string) error {
	dir := nearestExistingDir(filepath.Dir(path))
	if _, err := exec.LookPath("git"); err != nil {
		return nil //nolint:nilerr // git not installed: do not block the user
	}
	if !insideWorktree(dir) {
		return nil
	}
	cmd := exec.Command("git", "-C", dir, "check-ignore", "-q", "--", path)
	err := cmd.Run()
	if err == nil {
		return nil // exit 0: ignored
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return ErrNotIgnored // exit 1: not ignored
	}
	// exit 128 or other failure: git could not answer; do not block the user.
	return nil
}

// insideWorktree reports whether dir is inside a git working tree.
func insideWorktree(dir string) bool {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree").Output()
	if err != nil {
		return false
	}
	return bytes.Equal(bytes.TrimSpace(out), []byte("true"))
}

// nearestExistingDir walks up from dir until it finds a directory that exists.
func nearestExistingDir(dir string) string {
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}
