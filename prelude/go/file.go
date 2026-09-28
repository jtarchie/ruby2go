//go:build ignore

package prelude

// rbSysErr is MRI's Errno exception for a failed call: "<strerror> @ <MRI function> - <path>".
func rbSysErr(err error, fn, path string) any {
	msg := func(s string) *String { return Ref(String(s + " @ " + fn + " - " + path)) }
	switch { // errnos before fs.Err*: Go files ENOTEMPTY under ErrExist
	case errors.Is(err, syscall.ENOTEMPTY):
		return NewErrno_ENOTEMPTY(msg("Directory not empty"))
	case errors.Is(err, fs.ErrNotExist):
		return NewErrno_ENOENT(msg("No such file or directory"))
	case errors.Is(err, fs.ErrExist):
		return NewErrno_EEXIST(msg("File exists"))
	case errors.Is(err, fs.ErrPermission):
		return NewErrno_EACCES(msg("Permission denied"))
	case errors.Is(err, syscall.EISDIR):
		return NewErrno_EISDIR(msg("Is a directory"))
	case errors.Is(err, syscall.ENOTDIR):
		return NewErrno_ENOTDIR(msg("Not a directory"))
	}
	return NewIOError(Ref(String(err.Error())))
}

// rbStrs converts Go strings to an Array[String].
func rbStrs(ss []string) *Array[String] {
	a := make(Array[String], 0, len(ss))
	for _, s := range ss {
		a = append(a, String(s))
	}
	return &a
}

// rbTrimSlash drops trailing slashes but keeps a lone "/", as MRI's basename/dirname see paths.
func rbTrimSlash(p string) string {
	t := strings.TrimRight(p, "/")
	if t == "" && p != "" {
		return "/"
	}
	return t
}

// rbBasename is File.basename; suffix ".*" drops any extension.
func rbBasename(path, suffix string) string {
	p := rbTrimSlash(path)
	if p == "/" || p == "" {
		return p
	}
	b := p[strings.LastIndexByte(p, '/')+1:]
	switch {
	case suffix == ".*":
		if e := rbExtname(b); e != "" {
			b = b[:len(b)-len(e)]
		}
	case suffix != "" && suffix != b && strings.HasSuffix(b, suffix):
		b = b[:len(b)-len(suffix)]
	}
	return b
}

func rbDirname(path string) string {
	p := rbTrimSlash(path)
	i := strings.LastIndexByte(p, '/')
	switch {
	case p == "/":
		return "/"
	case i < 0:
		return "."
	}
	if d := strings.TrimRight(p[:i], "/"); d != "" {
		return d
	}
	return "/"
}

// rbExtname is File.extname: a leading dot is a hidden file, not an extension.
func rbExtname(path string) string {
	b := rbTrimSlash(path)
	b = b[strings.LastIndexByte(b, '/')+1:]
	i := strings.LastIndexByte(b, '.')
	if i <= 0 || strings.TrimLeft(b, ".") == "" {
		return ""
	}
	return b[i:]
}
