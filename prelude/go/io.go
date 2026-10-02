//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// next is ARGF's current reader: stdin when ARGV is empty at the first
// read, else the next ARGV file, shifted out of argv as MRI does. nil once
// everything is read.
func (self *ARGFClass) next(argv *Array[String]) *bufio.Reader {
	if self.r != nil || self.done {
		return self.r
	}
	if !self.started {
		self.started = true
		if len(*argv) == 0 {
			self.r, self.stdin, self.name = rbStdin, true, "-"
			return self.r
		}
	}
	if self.stdin || len(*argv) == 0 {
		self.done = true
		return nil
	}
	path := string((*argv)[0])
	*argv = (*argv)[1:]
	f, err := os.Open(path) //nolint:gosec // ARGV names the files to read, as in MRI
	if err != nil {
		panic(rbSysErr(err, "rb_sysopen", path))
	}
	self.f, self.r, self.name = f, bufio.NewReader(f), path
	return self.r
}

// spent closes the current source after it hit EOF.
func (self *ARGFClass) spent() {
	if self.f != nil {
		_ = self.f.Close()
		self.f = nil
	}
	self.r = nil
	if self.stdin {
		self.done = true
	}
}

// rbGetc is IO#getc: one UTF-8 character, or nil at end of file.
func rbGetc(r *bufio.Reader) *String {
	c, _, err := r.ReadRune()
	if err != nil {
		return nil
	}
	s := String(string(c))
	return &s
}

// rbGetbyte is IO#getbyte: one byte, or nil at end of file.
func rbGetbyte(r *bufio.Reader) *Integer {
	b, err := r.ReadByte()
	if err != nil {
		return nil
	}
	n := Integer(b)
	return &n
}

// rbEOF is readchar/readbyte's EOFError where getc/getbyte answer nil.
func rbEOF[T any](v *T) *T {
	if v == nil {
		panic(NewEOFError(Ref(String("end of file reached"))))
	}
	return v
}

// rbUngetc is IO#ungetc: s goes back in front of what is still unread.
func rbUngetc(r **bufio.Reader, s string) {
	*r = bufio.NewReader(io.MultiReader(strings.NewReader(s), *r))
}
