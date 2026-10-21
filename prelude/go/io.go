//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// next is ARGF's current reader: stdin when ARGV is empty at the first
// read, else the next ARGV file, shifted out of argv as MRI does. nil once
// everything is read.
func (self *ARGFClass) next(argv *Array[String]) *bufio.Reader {
	if self.r != nil || self.done {
		return self.r
	}
	if !self.started {
		self.started = true
		if len(argv.s) == 0 {
			self.r, self.stdin, self.name = rbStdin, true, "-"
			return self.r
		}
	}
	if self.stdin || len(argv.s) == 0 {
		self.done = true
		return nil
	}
	path := string(argv.s[0])
	argv.s = argv.s[1:]
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
	if self.noclose { // autoclose = false: the descriptor is the caller's; drop the buffers, keep the fd
		if self.w != nil {
			_ = self.w.Flush()
		}
		self.rf, self.r, self.wf, self.w = nil, nil, nil, nil
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

// rbDupFd is dup(2): a descriptor of the same open file, for an IO over a borrowed fd (autoclose: false).
func rbDupFd(fd int) int {
	nfd, err := syscall.Dup(fd)
	if err != nil {
		panic(rbSysErr(err, "dup", ""))
	}
	syscall.CloseOnExec(nfd)
	return nfd
}

// rbPinnedFiles never shrinks: an os.File's finalizer closes its fd and cannot be cleared, so unpinning a closed IO's file would close the borrowed fd later.
var (
	rbPinMu       sync.Mutex // a leaf: held only around the map, never across a callback
	rbPinnedFiles = map[*os.File]struct{}{}
)

// rbPinFiles keeps borrowed descriptors' os.Files reachable: their finalizer would close a fd another IO still owns.
func rbPinFiles(fs ...*os.File) {
	rbPinMu.Lock()
	defer rbPinMu.Unlock()
	for _, f := range fs {
		if f != nil {
			rbPinnedFiles[f] = struct{}{}
		}
	}
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
	// no mode is the descriptor's own; one it was not opened for is MRI's EINVAL (rb_io_initialize checks fcntl F_GETFL)
	fl, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFL, 0)
	has := int(fl) & syscall.O_ACCMODE
	if mode == "" {
		mode = map[int]string{syscall.O_RDONLY: "r", syscall.O_WRONLY: "w", syscall.O_RDWR: "r+"}[has]
		if errno != 0 || mode == "" {
			mode = "r"
		}
	}
	_, access := rbOpenAccess(mode)
	reads, writes := access == "r" || strings.HasSuffix(access, "+"), access != "r"
	if errno == 0 && (reads && has == syscall.O_WRONLY || writes && has == syscall.O_RDONLY) {
		panic(NewErrno_EINVAL(Ref(String("Invalid argument"))))
	}
	f := os.NewFile(uintptr(fd), "fd "+strconv.Itoa(fd))
	switch {
	case strings.HasSuffix(access, "+"):
		return rbIONew(f, f, nil)
	case access == "r":
		return rbIONew(f, nil, nil)
	}
	return rbIONew(nil, f, nil)
}

// rbReadNonblock reads up to n bytes that are there now: what self's buffer holds, else one read of its descriptor.
// With nonblock, an empty pipe raises IO::EAGAINWaitReadable (a read deadline); without, it waits for some.
// EOFError at the end, as MRI's read_nonblock and readpartial.
func rbReadNonblock(self *IO, n int, nonblock bool) String {
	if n < 0 {
		panic(NewArgumentError(Ref(String("negative length " + strconv.Itoa(n) + " given"))))
	}
	r := *self.rbReader()
	if n <= 0 {
		return ""
	}
	if r.Buffered() > 0 {
		buf := make([]byte, min(n, r.Buffered()))
		k, _ := r.Read(buf)
		return String(buf[:k])
	}
	f := self.rbOSFile()
	if nonblock { // a deadline already past is reported before the read is tried, hiding an EOF: a millisecond's grace
		_ = f.SetReadDeadline(time.Now().Add(time.Millisecond))
		defer func() { _ = f.SetReadDeadline(time.Time{}) }()
	}
	buf := make([]byte, n)
	k, err := r.Read(buf)
	if k > 0 {
		return String(buf[:k])
	}
	switch {
	case errors.Is(err, os.ErrDeadlineExceeded):
		panic(NewIO_EAGAINWaitReadable(Ref(String("Resource temporarily unavailable - read would block"))))
	case errors.Is(err, io.EOF) || err == nil:
		panic(NewEOFError(Ref(String("end of file reached"))))
	}
	panic(rbSysErr(err, "rb_io_read_nonblock", ""))
}

// rbIOSelect is IO.select over readers: those with buffered bytes, data or an end to report, once any has; nil when
// limit passes first. Each is probed by a Peek under a short read deadline, which consumes nothing.
func rbIOSelect(ios []*IO, limit *time.Duration) []*IO {
	var stop time.Time
	if limit != nil {
		stop = time.Now().Add(*limit)
	}
	for {
		var ready []*IO
		for _, x := range ios {
			r := *x.rbReader()
			if r.Buffered() > 0 {
				ready = append(ready, x)
				continue
			}
			f := x.rbOSFile()
			_ = f.SetReadDeadline(time.Now().Add(time.Millisecond))
			_, err := r.Peek(1)
			_ = f.SetReadDeadline(time.Time{})
			if !errors.Is(err, os.ErrDeadlineExceeded) {
				ready = append(ready, x) // data, or the end (EOF), or an error a read reports
			}
		}
		if len(ready) > 0 {
			return ready
		}
		if limit != nil && !time.Now().Before(stop) {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// rbWinsize is TIOCGWINSZ on f: its rows and columns.
func rbWinsize(f *os.File) ([4]uint16, error) {
	var ws [4]uint16
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws))) //nolint:gosec // the ioctl's out-parameter
	if e != 0 {
		return ws, e
	}
	return ws, nil
}

// rbReadRest reads r to its end, as io.ReadAll, but into a buffer sized from f (the file r reads, or nil) when it is a
// regular file: one allocation instead of ReadAll's doublings, and strings.Builder hands it over without a copy.
func rbReadRest(r io.Reader, f *os.File) String {
	var b strings.Builder
	if f != nil {
		if fi, err := f.Stat(); err == nil && fi.Mode().IsRegular() {
			b.Grow(int(fi.Size()))
		}
	}
	_, _ = io.Copy(&b, r)
	return String(b.String())
}
