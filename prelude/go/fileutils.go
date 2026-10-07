//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo); types like File come from generated code.
package prelude

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func rbCopyFile(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return syscall.EISDIR
	}
	in, err := os.Open(src) //nolint:gosec // path is Ruby-controlled, same trust boundary as File.read elsewhere
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode().Perm()) //nolint:gosec // MRI's mode; the umask applies
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil { //nolint:gosec // Ruby-controlled path, not attacker-supplied length
		_ = out.Close()
		return err
	}
	return out.Close()
}

// rbCopyTree recreates src's tree under dst; symlinks are relinked, not followed.
func rbCopyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path) //nolint:gosec // cp_r walks the caller's own tree, as MRI's does
			if err != nil {
				return err
			}
			return os.Symlink(link, target) //nolint:gosec // as above
		case d.IsDir():
			fi, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, fi.Mode().Perm()) //nolint:gosec // as above
		default:
			return rbCopyFile(path, target)
		}
	})
}

// rbMove is os.Rename with a copy+remove fallback across devices, like MRI's FileUtils.mv.
func rbMove(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil || !errors.Is(err, syscall.EXDEV) {
		return err
	}
	fi, serr := os.Stat(src)
	if serr != nil {
		return serr
	}
	if fi.IsDir() {
		if cerr := rbCopyTree(src, dst); cerr != nil {
			return cerr
		}
	} else if cerr := rbCopyFile(src, dst); cerr != nil {
		return cerr
	}
	return os.RemoveAll(src)
}

// rbTouch creates path if missing, else bumps its mtime/atime to now, like MRI's FileUtils.touch.
func rbTouch(path string) error {
	now := time.Now()
	if err := os.Chtimes(path, now, now); err == nil {
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o666) //nolint:gosec // MRI's mode; the umask applies
	if err != nil {
		return err
	}
	return f.Close()
}
