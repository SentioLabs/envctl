// Package filesink writes secret material to files with restrictive modes and
// atomic replacement. It has no knowledge of secrets backends or config.
package filesink

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotIgnored is returned when path is inside a git worktree and not ignored by git.
var ErrNotIgnored = errors.New("path is inside a git worktree and not ignored by git")

// dirMode is the mode of every directory this package creates.
const dirMode os.FileMode = 0o700

// Dir is a directory that receives sink files.
type Dir struct {
	path      string
	ephemeral bool
	closed    bool
}

// Path returns the directory's absolute path.
func (d *Dir) Path() string { return d.path }

// NewEphemeral creates a 0700 directory under os.TempDir(). Close removes it.
func NewEphemeral() (*Dir, error) {
	path, err := os.MkdirTemp("", "envctl-")
	if err != nil {
		return nil, fmt.Errorf("create run directory: %w", err)
	}
	// MkdirTemp already uses 0700, but make it explicit so a permissive umask can't widen it.
	if err := os.Chmod(path, dirMode); err != nil {
		_ = os.RemoveAll(path)
		return nil, fmt.Errorf("restrict run directory: %w", err)
	}
	return &Dir{path: path, ephemeral: true}, nil
}

// Close removes an ephemeral directory and its contents. Safe to call twice.
// Persistent directories are left alone.
func (d *Dir) Close() error {
	if d == nil || d.closed || !d.ephemeral {
		return nil
	}
	d.closed = true
	return os.RemoveAll(d.path)
}

// WriteEphemeral writes content as name inside d with mode, atomically.
// Returns the absolute path of the written file.
func (d *Dir) WriteEphemeral(name string, content []byte, mode os.FileMode) (string, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid sink file name %q", name)
	}
	root, err := os.OpenRoot(d.path)
	if err != nil {
		return "", fmt.Errorf("open run directory: %w", err)
	}
	defer root.Close()
	if err := writeAtomic(root, name, content, mode); err != nil {
		return "", err
	}
	return filepath.Join(d.path, name), nil
}

// Expand resolves ~, ${VAR}, and relative paths against configDir.
func Expand(path, configDir string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	p := os.ExpandEnv(path)
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand ~: %w", err)
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(configDir, p)
	}
	return filepath.Clean(p), nil
}

// WritePersistent expands path against configDir, refuses an unignored path
// inside a git worktree, creates the parent directory 0700 if missing, and
// writes atomically. Returns the absolute path.
func WritePersistent(path, configDir string, content []byte, mode os.FileMode) (string, error) {
	abs, err := Expand(path, configDir)
	if err != nil {
		return "", err
	}
	if err := CheckPath(abs); err != nil {
		return "", fmt.Errorf("%s: %w", abs, err)
	}
	parent := filepath.Dir(abs)
	if err := os.MkdirAll(parent, dirMode); err != nil {
		return "", fmt.Errorf("create %s: %w", parent, err)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", parent, err)
	}
	defer root.Close()
	if err := writeAtomic(root, filepath.Base(abs), content, mode); err != nil {
		return "", err
	}
	return abs, nil
}

// writeAtomic writes content to a temp file inside root and renames it over name.
// Every open and rename is confined to root, so a symlink swapped in under the
// directory can't redirect the write. A failure leaves no temp file behind.
func writeAtomic(root *os.Root, name string, content []byte, mode os.FileMode) error {
	tmp := "." + name + ".tmp-" + randomSuffix()
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	cleanup := func() { _ = root.Remove(tmp) }

	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close %s: %w", name, err)
	}
	// O_CREATE honors umask; force the requested mode.
	if err := root.Chmod(tmp, mode); err != nil {
		cleanup()
		return fmt.Errorf("chmod %s: %w", name, err)
	}
	if err := root.Rename(tmp, name); err != nil {
		cleanup()
		return fmt.Errorf("rename %s: %w", name, err)
	}
	return nil
}

func randomSuffix() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is not recoverable; fall back to a fixed suffix
		// and let O_EXCL reject a collision.
		return "00000000"
	}
	return hex.EncodeToString(b[:])
}
