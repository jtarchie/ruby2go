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

// rbGlob is Dir.glob: `*`, `?`, `[set]` per path segment (a leading dot
// only matched by a dot), `**/` for any depth of non-hidden directories,
// `{a,b}` alternatives expanded in order, and a trailing `/` for
// directories only. Each directory's entries are visited in sorted order,
// a match before its subtree, as MRI's sort: true default gives.
func rbGlob(pattern string) []string {
	var out []string
	for _, p := range rbBraces(pattern) {
		dir, prefix := ".", ""
		if strings.HasPrefix(p, "/") {
			dir, prefix, p = "/", "/", strings.TrimLeft(p, "/")
		}
		segs := strings.Split(p, "/")
		for i, s := range segs {
			if s == "**" && i == len(segs)-1 {
				segs[i] = "*" // `a/**` is `a/*`
			}
		}
		rbGlobWalk(dir, prefix, segs, &out)
	}
	return out
}

// rbBraces expands the first top-level {a,b} of p, recursively.
func rbBraces(p string) []string {
	depth, start := 0, -1
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '\\':
			i++
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth > 0 {
				continue
			}
			var alts []string
			d, last := 0, start+1
			for j := start + 1; j < i; j++ {
				switch p[j] {
				case '\\':
					j++
				case '{':
					d++
				case '}':
					d--
				case ',':
					if d == 0 {
						alts = append(alts, p[last:j])
						last = j + 1
					}
				}
			}
			alts = append(alts, p[last:i])
			out := make([]string, 0, len(alts))
			for _, a := range alts {
				out = append(out, rbBraces(p[:start]+a+p[i+1:])...)
			}
			return out
		}
	}
	return []string{p}
}

func rbGlobMatch(seg, name string) bool {
	if strings.HasPrefix(name, ".") && !strings.HasPrefix(seg, ".") {
		return false
	}
	ok, _ := filepath.Match(seg, name)
	return ok
}

func rbGlobWalk(dir, prefix string, segs []string, out *[]string) {
	seg, rest := segs[0], segs[1:]
	if seg == "" { // a trailing slash: dir itself, already known to be one
		*out = append(*out, prefix)
		return
	}
	if seg != "**" && !strings.ContainsAny(seg, "*?[\\") {
		p := filepath.Join(dir, seg)
		fi, err := os.Stat(p)
		if err != nil {
			return
		}
		rbGlobStep(p, prefix+seg, fi.IsDir(), rest, out)
		return
	}
	es, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	names := make([]string, 0, len(es))
	for _, e := range es {
		names = append(names, e.Name())
	}
	slices.Sort(names)
	for _, name := range names {
		p := filepath.Join(dir, name)
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if seg == "**" {
			if rbGlobMatch(rest[0], name) || rest[0] == "" && !strings.HasPrefix(name, ".") {
				if rest[0] == "" {
					if fi.IsDir() {
						*out = append(*out, prefix+name+"/")
					}
				} else {
					rbGlobStep(p, prefix+name, fi.IsDir(), rest[1:], out)
				}
			}
			if fi.IsDir() && !strings.HasPrefix(name, ".") {
				rbGlobWalk(p, prefix+name+"/", segs, out)
			}
			continue
		}
		if rbGlobMatch(seg, name) {
			rbGlobStep(p, prefix+name, fi.IsDir(), rest, out)
		}
	}
}

// rbGlobStep continues past a matched entry: emit it, or descend.
func rbGlobStep(p, shown string, isDir bool, rest []string, out *[]string) {
	switch {
	case len(rest) == 0:
		*out = append(*out, shown)
	case isDir:
		rbGlobWalk(p, shown+"/", rest, out)
	}
}

// rbSourceDir is Kernel#__dir__: the source file's absolute directory, resolved against the working directory as MRI does at load.
func rbSourceDir(name string) String {
	p, err := filepath.Abs(name)
	if err != nil {
		return String(filepath.Dir(name))
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return String(filepath.Dir(p))
}
