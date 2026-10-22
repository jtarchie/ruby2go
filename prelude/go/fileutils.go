//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo); types like File come from generated code.
package prelude

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// rbArgError is a refusal MRI reports as ArgumentError (same file, directory into itself), not an errno.
type rbArgError struct{ msg string }

func (e *rbArgError) Error() string { return e.msg }

// rbFileErr is the exception for an error from the helpers below: ArgumentError for a refusal, else the errno class.
func rbFileErr(err error, fn, path string) any {
	var ae *rbArgError
	if errors.As(err, &ae) {
		return NewArgumentError(Ref(String(ae.msg)))
	}
	return rbSysErr(err, fn, path)
}

// rbSameFile refuses src == dst through links: opening dst with O_TRUNC would empty src before it is read.
func rbSameFile(src, dst string) error {
	si, serr := os.Stat(src)
	di, derr := os.Stat(dst)
	if serr == nil && derr == nil && os.SameFile(si, di) {
		return &rbArgError{"same file: " + src + " and " + dst}
	}
	return nil
}

// rbResolve is EvalSymlinks of path's deepest existing ancestor plus the rest, so a not-yet-created dst compares by real location.
func rbResolve(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	rest := ""
	for p := abs; ; p = filepath.Dir(p) {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(r, rest)
		}
		if filepath.Dir(p) == p {
			return abs
		}
		rest = filepath.Join(filepath.Base(p), rest)
	}
}

func rbCopyFile(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return syscall.EISDIR
	}
	if err := rbSameFile(src, dst); err != nil {
		return err
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

// rbCopyTree recreates src's tree under dst; symlinks are relinked, not followed. As MRI's copy_entry, directories are made
// with the default mode (the umask applies) so a read-only source still takes its children; preserve then sets each
// directory's mode once its children are in.
func rbCopyTree(src, dst string, preserve bool) error {
	if rs, rd := rbResolve(src), rbResolve(dst); rd == rs || strings.HasPrefix(rd, rs+string(filepath.Separator)) {
		abs, _ := filepath.Abs(src)
		return &rbArgError{"cannot copy directory " + abs + " to itself " + dst}
	}
	var dirs [][2]string
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
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
			if preserve {
				dirs = append(dirs, [2]string{path, target})
			}
			return os.MkdirAll(target, 0o777) //nolint:gosec // as above
		default:
			if err := rbCopyFile(path, target); err != nil || !preserve {
				return err
			}
			return rbKeep(path, target)
		}
	})
	for i := len(dirs) - 1; err == nil && i >= 0; i-- {
		err = rbKeep(dirs[i][0], dirs[i][1])
	}
	return err
}

// rbKeep gives dst src's mode (special bits too) and times: MRI's preserve_attributes, minus the owner.
func rbKeep(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.Chmod(dst, fi.Mode()&(fs.ModePerm|fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky)); err != nil {
		return err
	}
	return os.Chtimes(dst, rbAtime(fi), fi.ModTime())
}

// rbMove is os.Rename with a copy+remove fallback across devices, like MRI's FileUtils.mv.
func rbMove(src, dst string) error {
	if err := rbSameFile(src, dst); err != nil {
		return err
	}
	err := os.Rename(src, dst)
	if err == nil || !errors.Is(err, syscall.EXDEV) {
		return err
	}
	fi, serr := os.Stat(src)
	if serr != nil {
		return serr
	}
	if fi.IsDir() {
		if cerr := rbCopyTree(src, dst, true); cerr != nil {
			return cerr
		}
	} else if cerr := rbCopyFile(src, dst); cerr != nil {
		return cerr
	}
	return os.RemoveAll(src)
}

// rbTouch is FileUtils.touch: sets both times to t, creating path unless nocreate (then a missing path is an error).
func rbTouch(path string, t time.Time, nocreate bool) error {
	err := os.Chtimes(path, t, t)
	if err == nil || nocreate {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o666) //nolint:gosec // MRI's mode; the umask applies
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Chtimes(path, t, t)
}

// rbSameContents is FileUtils.compare_file; an unreadable file raises as MRI's File.open does.
func rbSameContents(a, b string) bool {
	fa, err := os.Open(a) //nolint:gosec // the program's own paths
	if err != nil {
		panic(rbSysErr(err, "rb_sysopen", a))
	}
	fb, err := os.Open(b) //nolint:gosec // the program's own paths
	if err != nil {
		panic(rbSysErr(err, "rb_sysopen", b))
	}
	defer func() { _ = fa.Close(); _ = fb.Close() }()
	sa, ea := fa.Stat()
	sb, eb := fb.Stat()
	if ea == nil && eb == nil && sa.Mode().IsRegular() && sb.Mode().IsRegular() && sa.Size() != sb.Size() {
		return false // as MRI: sizes first, then the bytes
	}
	ba, bb := make([]byte, 64<<10), make([]byte, 64<<10)
	for {
		na, erra := io.ReadFull(fa, ba)
		nb, errb := io.ReadFull(fb, bb)
		if erra != nil && !errors.Is(erra, io.EOF) && !errors.Is(erra, io.ErrUnexpectedEOF) {
			panic(rbSysErr(erra, "rb_sysopen", a))
		}
		if errb != nil && !errors.Is(errb, io.EOF) && !errors.Is(errb, io.ErrUnexpectedEOF) {
			panic(rbSysErr(errb, "rb_sysopen", b))
		}
		if na != nb || !bytes.Equal(ba[:na], bb[:nb]) {
			return false
		}
		if na < len(ba) {
			return true
		}
	}
}
