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

// rbIONew is a pipe or popen IO over the given ends; it shows the read end's descriptor, as MRI's inspect does.
func rbIONew(rf, wf *os.File, cmd *exec.Cmd) *IO {
	x := &IO{own: true, rf: rf, wf: wf, cmd: cmd, sync: wf != nil || cmd != nil}
	if wf != nil {
		x.w, x.fd = bufio.NewWriter(wf), rbFileFd(wf)
	}
	if rf != nil {
		x.r, x.fd = bufio.NewReader(rf), rbFileFd(rf)
	}
	return x
}

// rbFileFd reads f's descriptor through SyscallConn, since os.File.Fd would switch it to blocking mode.
func rbFileFd(f *os.File) int {
	n := -1
	if rc, err := f.SyscallConn(); err == nil {
		_ = rc.Control(func(fd uintptr) { n = int(fd) })
	}
	return n
}

func (self *IO) rbOpen() {
	if self.closed {
		panic(NewIOError(Ref[String]("closed stream")))
	}
}

// rbOSFile is the descriptor behind self, for tty? and PP's width.
func (self *IO) rbOSFile() *os.File {
	switch {
	case !self.own:
		return []*os.File{os.Stdin, os.Stdout, os.Stderr}[self.fd]
	case self.wf != nil:
		return self.wf
	}
	return self.rf
}

// rbReader is a pointer so ungetc can replace the reader: STDIN's shared one, or the pipe's.
func (self *IO) rbReader() **bufio.Reader {
	self.rbOpen()
	switch {
	case self.own && self.r != nil:
		return &self.r
	case !self.own && self.fd == 0:
		return &rbStdin
	}
	panic(NewIOError(Ref[String]("not opened for reading")))
}

func (self *IO) rbWritePipe(s string) {
	if self.w == nil {
		panic(NewIOError(Ref[String]("not opened for writing")))
	}
	_, err := self.w.WriteString(s)
	if err == nil && self.sync {
		err = self.w.Flush()
	}
	if err != nil {
		panic(rbIOWriteErr(err))
	}
}

// rbIOWriteErr: Go ignores SIGPIPE off stdout, so a write after the reader is gone comes back as EPIPE, MRI's Errno::EPIPE.
func rbIOWriteErr(err error) any {
	if errors.Is(err, syscall.EPIPE) {
		return NewErrno_EPIPE(Ref(String("Broken pipe")))
	}
	return NewIOError(Ref(String(err.Error())))
}

// rbClose reaps a popen child into $? after closing its pipes, so the child sees EOF first; a second close is a no-op, as MRI's.
func (self *IO) rbClose() {
	if self.closed {
		return
	}
	self.closed = true
	if !self.own {
		if self.fd == 1 {
			rbFlush()
		}
		return
	}
	self.rbCloseRead()
	self.rbCloseWrite()
	if self.cmd != nil {
		rbPopenReap(self.cmd)
	}
}

func (self *IO) rbCloseRead() {
	if self.rf != nil {
		_ = self.rf.Close()
		self.rf, self.r = nil, nil
	}
}

func (self *IO) rbCloseWrite() {
	if self.wf != nil {
		if self.w != nil {
			_ = self.w.Flush()
		}
		_ = self.wf.Close()
		self.wf, self.w = nil, nil
	}
}

// rbCloseHalf follows MRI: popen "r+" closes one pipe at a time, a popen or the side an IO holds closes it whole, the other side of a pipe raises.
func (self *IO) rbCloseHalf(read bool) {
	if self.closed {
		return
	}
	mine := self.rf != nil
	if !read {
		mine = self.wf != nil
	}
	if !self.own {
		mine = (self.fd == 0) == read
	}
	switch {
	case self.own && self.rf != nil && self.wf != nil:
		if read {
			self.rbCloseRead()
			self.fd = rbFileFd(self.wf) // MRI's IO takes over the write side's descriptor
		} else {
			self.rbCloseWrite()
		}
	case mine || self.cmd != nil:
		self.rbClose()
	case read:
		panic(NewIOError(Ref[String]("closing non-duplex IO for reading")))
	default:
		panic(NewIOError(Ref[String]("closing non-duplex IO for writing")))
	}
}

// rbOpenAccess is a mode string's open(2) flags: "r", "w", "a", their "+"
// forms, with "b"/"t" and any ":enc" ignored.
func rbOpenAccess(mode string) (int, string) {
	access, _, _ := strings.Cut(mode, ":")
	access = strings.NewReplacer("b", "", "t", "").Replace(access)
	flags := map[string]int{
		"r": os.O_RDONLY, "r+": os.O_RDWR,
		"w": os.O_WRONLY | os.O_CREATE | os.O_TRUNC, "w+": os.O_RDWR | os.O_CREATE | os.O_TRUNC,
		"a": os.O_WRONLY | os.O_CREATE | os.O_APPEND, "a+": os.O_RDWR | os.O_CREATE | os.O_APPEND,
	}
	flag, ok := flags[access]
	if !ok {
		panic(NewArgumentError(Ref(String("invalid access mode " + mode))))
	}
	return flag, access
}

// rbSysopen is IO.sysopen: a raw descriptor no Go *os.File owns, so no
// finalizer closes it under the program.
func rbSysopen(path, mode string, perm int) int {
	flag, _ := rbOpenAccess(mode)
	fd, err := syscall.Open(path, flag|syscall.O_CLOEXEC, uint32(perm)) //nolint:gosec // MRI's mode; the umask applies
	if err != nil {
		panic(rbSysErr(err, "rb_sysopen", path))
	}
	return fd
}

// rbIOForFd is IO.new(fd, mode): the standard streams for 0-2, else an IO
// that owns fd (closing it closes fd, as MRI's autoclose).
func rbIOForFd(fd int, mode string) *IO {
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		panic(rbSysErr(err, "rb_io_initialize", ""))
	}
	if fd <= 2 {
		return &IO{fd: fd}
	}
	_, access := rbOpenAccess(mode)
	f := os.NewFile(uintptr(fd), "fd "+strconv.Itoa(fd))
	switch {
	case strings.HasSuffix(access, "+"):
		return rbIONew(f, f, nil)
	case access == "r":
		return rbIONew(f, nil, nil)
	}
	return rbIONew(nil, f, nil)
}
