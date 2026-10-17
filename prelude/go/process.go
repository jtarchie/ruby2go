//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// rbProcessStart anchors the monotonic clock; Go's time.Since reads the monotonic reading it carries.
var rbProcessStart = time.Now()

// rbClockGettimeUnit is clock_gettime(id, unit): MRI's units, a Float for :float_* and an Integer otherwise.
func rbClockGettimeUnit(id int, unit string) any {
	s := float64(rbClockGettime(id))
	switch unit {
	case "float_second":
		return Float(s)
	case "float_millisecond":
		return Float(s * 1e3)
	case "float_microsecond":
		return Float(s * 1e6)
	case "second":
		return Integer(s)
	case "millisecond":
		return Integer(s * 1e3)
	case "microsecond":
		return Integer(s * 1e6)
	case "nanosecond":
		return Integer(s * 1e9)
	}
	panic(NewArgumentError(Ref(String("unexpected unit: " + unit))))
}

func rbClockGettime(id int) Float {
	switch id {
	case 0:
		return Float(float64(time.Now().UnixNano()) / 1e9)
	case 6:
		return Float(time.Since(rbProcessStart).Seconds())
	case 12:
		var ru syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
			panic(NewStandardError(Ref(String(err.Error()))))
		}
		return Float(time.Duration(ru.Utime.Nano() + ru.Stime.Nano()).Seconds())
	}
	panic(NewErrno_EINVAL(Ref(String(fmt.Sprintf("Invalid argument - clock_gettime(%d)", id)))))
}

// rbLastStatus is `$?`: the status of the last system or backtick command.
// ponytail: one for the program, where MRI's is per thread.
var rbLastStatus atomic.Pointer[Process_Status]

// rbShellMeta are the characters that make MRI hand a command string to
// /bin/sh rather than exec its words directly.
const rbShellMeta = "*?{}[]<>()~&|\\$;'`\"\n#="

// rbShellWords are the reserved words and special built-ins MRI also sends to sh.
var rbShellWords = map[string]bool{
	"!": true, ".": true, ":": true, "break": true, "case": true, "continue": true, "do": true, "done": true,
	"elif": true, "else": true, "esac": true, "eval": true, "exec": true, "exit": true, "export": true,
	"fi": true, "for": true, "if": true, "in": true, "readonly": true, "return": true, "set": true,
	"shift": true, "then": true, "times": true, "trap": true, "unset": true, "until": true, "while": true,
}

// rbCommand builds a system/backtick command as MRI does: several
// arguments exec directly, one string goes through sh -c when it has
// shell syntax and is split on whitespace otherwise.
// rbNoNul is MRI's StringValueCStr check.
func rbNoNul(s string) {
	if strings.IndexByte(s, 0) >= 0 {
		panic(NewArgumentError(Ref(String("string contains null byte"))))
	}
}

func rbCommand(cmd string, args []String) *exec.Cmd {
	for _, s := range append([]String{String(cmd)}, args...) {
		rbNoNul(string(s))
	}
	if len(args) > 0 {
		rest := make([]string, len(args))
		for i, a := range args {
			rest[i] = string(a)
		}
		return exec.Command(cmd, rest...) //nolint:gosec // running the program's command is the point
	}
	words := strings.Fields(cmd)
	if len(words) == 0 {
		words = []string{""}
	}
	if strings.ContainsAny(cmd, rbShellMeta) || rbShellWords[words[0]] {
		return exec.Command("/bin/sh", "-c", cmd) //nolint:gosec // as MRI, a string with shell syntax goes to sh
	}
	return exec.Command(words[0], words[1:]...) //nolint:gosec // running the program's command is the point
}

// rbRunCommand runs c after flushing stdout, as MRI does before a spawn,
// and records $?. started is false when the program could not be run.
func rbRunCommand(c *exec.Cmd) (started bool) {
	rbFlush()
	err := c.Run()
	var ee *exec.ExitError
	if err != nil && !errors.As(err, &ee) {
		rbLastStatus.Store(&Process_Status{code: 127})
		return false
	}
	rbLastStatus.Store(rbStatusOf(c))
	return true
}

// rbStatusOf reads a waited command's exit code, or the signal that ended it (ExitCode is -1 then), into a Process::Status.
func rbStatusOf(c *exec.Cmd) *Process_Status {
	st := &Process_Status{}
	if c.Process != nil {
		st.pid = c.Process.Pid
	}
	if ps := c.ProcessState; ps != nil {
		if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			st.sig, st.core = int(ws.Signal()), ws.CoreDump()
		} else {
			st.code = ps.ExitCode()
		}
	}
	return st
}

// rbStatusString is Process::Status#to_s: "pid 7 exit 3", or "pid 7 SIGKILL (signal 9)".
func rbStatusString(st *Process_Status) String {
	if st.sig != 0 {
		return String(fmt.Sprintf("pid %d SIG%s (signal %d)", st.pid, rbSignalName(syscall.Signal(st.sig)), st.sig))
	}
	return String(fmt.Sprintf("pid %d exit %d", st.pid, st.code))
}

func rbSystem(cmd string, args []String) *Boolean {
	c := rbCommand(cmd, args)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if !rbRunCommand(c) {
		return nil
	}
	st := rbLastStatus.Load()
	return Ref(Boolean(st.sig == 0 && st.code == 0))
}

func rbBacktick(cmd string) String {
	c := rbCommand(cmd, nil)
	var out strings.Builder
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, &out, os.Stderr
	if !rbRunCommand(c) {
		panic(NewErrno_ENOENT(Ref(String("No such file or directory - " + cmd))))
	}
	return String(out.String())
}

// rbChildren are the processes Process.spawn started and nothing has
// waited for yet, by pid; each has a goroutine waiting on it that closes
// done (decision 107).
var (
	rbChildrenMu sync.Mutex
	rbChildren   = map[int]*rbChild{}
)

type rbChild struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

// rbSpawn starts a child with the program's standard streams and answers its pid.
func rbSpawn(cmd string, args []String) Integer {
	c := rbCommand(cmd, args)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return rbStartChild(c, nil)
}

// rbStartChild flushes stdout, starts c, closes the parent's copies of its redirection files and registers it for Process.wait (decision 107).
func rbStartChild(c *exec.Cmd, closers []*os.File) Integer {
	rbFlush()
	err := c.Start()
	for _, f := range closers {
		_ = f.Close()
	}
	if err != nil {
		panic(rbExecErr(err, c.Path))
	}
	ch := &rbChild{cmd: c, done: make(chan struct{})}
	rbChildrenMu.Lock()
	rbChildren[c.Process.Pid] = ch
	rbChildrenMu.Unlock()
	go func() {
		ch.err = c.Wait()
		close(ch.done)
	}()
	return Integer(c.Process.Pid)
}

// rbWait reaps pid (-1: the first child to finish), records $? and answers the pid.
func rbWait(pid int) Integer {
	rbChildrenMu.Lock()
	var ch *rbChild
	if pid >= 0 {
		ch = rbChildren[pid]
	} else {
		for p, c := range rbChildren { // any finished one, else any
			if ch == nil {
				pid, ch = p, c
			}
			select {
			case <-c.done:
				pid, ch = p, c
			default:
			}
		}
	}
	if ch != nil {
		delete(rbChildren, pid)
	}
	rbChildrenMu.Unlock()
	if ch == nil {
		panic(NewErrno_ECHILD(Ref(String("No child processes"))))
	}
	<-ch.done
	rbLastStatus.Store(rbStatusOf(ch.cmd))
	return Integer(pid)
}

// rbExec replaces the process with the command, as MRI's exec; stdout is flushed first.
func rbExec(cmd string, args []String) {
	c := rbCommand(cmd, args)
	if c.Err != nil {
		panic(NewErrno_ENOENT(Ref(String("No such file or directory - " + cmd))))
	}
	rbFlush()
	if err := syscall.Exec(c.Path, c.Args, os.Environ()); err != nil { //nolint:gosec // Kernel#exec: replacing the process with the program's command is the point
		panic(NewErrno_ENOENT(Ref(String("No such file or directory - " + cmd))))
	}
}

// rbRlimit reads Process.getrlimit's resource argument.
func rbRlimit(v any) int {
	names := map[string]int{"AS": syscall.RLIMIT_AS, "CORE": syscall.RLIMIT_CORE, "CPU": syscall.RLIMIT_CPU,
		"DATA": syscall.RLIMIT_DATA, "FSIZE": syscall.RLIMIT_FSIZE, "NOFILE": syscall.RLIMIT_NOFILE, "STACK": syscall.RLIMIT_STACK}
	switch r := rbUnbox(v).(type) {
	case Integer:
		return int(r)
	case Symbol, String:
		if n, ok := names[fmt.Sprint(r)]; ok {
			return n
		}
	}
	panic(NewArgumentError(Ref(String("invalid resource name: " + string(rbToS(v))))))
}

// rbPopen starts IO.popen's command with a pipe on its stdout ("r"), stdin ("w") or both ("r+"/"w+"); the rest stay the program's.
func rbPopen(cmd any, mode string, opts *Hash[Symbol, any]) *IO {
	m := strings.Map(func(r rune) rune {
		if r == 'b' || r == 't' {
			return -1
		}
		return r
	}, mode)
	read, write := m == "r" || m == "r+" || m == "w+", m == "w" || m == "r+" || m == "w+"
	if !read && !write {
		panic(NewArgumentError(Ref(String("invalid access mode " + mode))))
	}
	var args []any // spawn's arguments: [env,] program, its arguments (an Array command spreads; a String is one)
	switch v := rbUnbox(cmd).(type) {
	case String:
		args = []any{v}
	case Array_Any:
		args = v._ToAny().s
		if len(args) > 0 && len(opts.keys) == 0 { // `IO.popen([cmd, ..., opts])`: trailing options inside the Array
			if h, ok := rbUnbox(args[len(args)-1]).(Hash_Any); ok {
				hh, o := h._ToAny(), NewHash[Symbol, any]()
				for _, k := range hh.keys {
					if s, ok := rbUnbox(k).(Symbol); ok {
						Hash___Set(o, s, hh.vals[k])
					}
				}
				args, opts = args[:len(args)-1], o
			}
		}
	default:
		panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(cmd) + " into String"))))
	}
	c, theirs := rbSpawnCmd(args, opts)
	var ours []*os.File // our pipe ends; theirs are the child's, closed in the parent once it starts
	pipe := func() (*os.File, *os.File) {
		r, w, err := os.Pipe()
		if err != nil {
			for _, f := range append(ours, theirs...) {
				_ = f.Close()
			}
			panic(rbSysErr(err, "rb_io_s_popen", ""))
		}
		return r, w
	}
	var rf, wf *os.File
	if read {
		r, w := pipe()
		rf, c.Stdout = r, w
		ours, theirs = append(ours, r), append(theirs, w)
	}
	if write {
		r, w := pipe()
		wf, c.Stdin = w, r
		ours, theirs = append(ours, w), append(theirs, r)
	}
	rbFlush()
	err := c.Start()
	for _, f := range theirs {
		_ = f.Close()
	}
	if err != nil {
		for _, f := range ours {
			_ = f.Close()
		}
		panic(rbExecErr(err, c.Path))
	}
	return rbIONew(rf, wf, c)
}

// rbPopenReap waits for popen's child, its pipes already closed, and records $?.
func rbPopenReap(c *exec.Cmd) {
	_ = c.Wait()
	rbLastStatus.Store(rbStatusOf(c))
}

// rbLastStatusOpt is $?: the last child's status, nil before any.
func rbLastStatusOpt() **Process_Status {
	if st := rbLastStatus.Load(); st != nil {
		return &st
	}
	return nil
}

// rbSpawnFull is Process.spawn's general form (decision 233): an optional env Hash (nil unsets), the command as one
// String (decision 97's shell rule), as a program and its arguments, or as [program, argv0] and its arguments; then
// the options it honours: in:/out:/err: (an IO, a path String, or :close), chdir:, pgroup:, unsetenv_others:.
// ponytail: other options (umask:, close_others:, an Integer fd key) are accepted and ignored.
func rbSpawnFull(args []any, opts *Hash[Symbol, any]) Integer {
	c, closers := rbSpawnCmd(args, opts)
	return rbStartChild(c, closers)
}

// rbSpawnCmd is the command rbSpawnFull and rbPopen start, not yet started; closers are the path redirections it opened, for the caller to close once the child holds its copies.
func rbSpawnCmd(args []any, opts *Hash[Symbol, any]) (c *exec.Cmd, closers []*os.File) {
	defer func() {
		if r := recover(); r != nil { // a later option failed: release what earlier ones opened
			for _, f := range closers {
				_ = f.Close()
			}
			panic(r)
		}
	}()
	redirect := func(v any, flag int, reading bool) *os.File {
		f, owned := rbSpawnFile(v, flag, reading)
		if owned {
			closers = append(closers, f)
		}
		return f
	}
	var env map[string]any
	if len(args) > 0 {
		if h, ok := args[0].(Hash_Any); ok {
			env = map[string]any{}
			hh := h._ToAny()
			for _, k := range hh.keys {
				name, v := string(rbToS(k)), rbUnbox(hh.vals[k])
				rbNoNul(name)
				if strings.Contains(name, "=") {
					panic(NewArgumentError(Ref(String("environment name contains a equal : " + name))))
				}
				if s, ok := v.(String); ok {
					rbNoNul(string(s))
				}
				env[name] = v
			}
			args = args[1:]
		}
	}
	if len(args) == 0 {
		panic(NewArgumentError(Ref(String("wrong number of arguments (given 0, expected 1+)"))))
	}
	argv0 := ""
	rest := make([]String, 0, len(args)-1)
	for _, a := range args[1:] {
		rest = append(rest, rbToS(a))
	}
	switch first := rbUnbox(args[0]).(type) {
	case Array_Any:
		pair := first._ToAny().s
		if len(pair) != 2 {
			panic(NewArgumentError(Ref(String("wrong first argument"))))
		}
		argv0 = string(rbToS(pair[1]))
		rbNoNul(argv0)
		c = rbCommand(string(rbToS(pair[0])), rest)
		if len(rest) == 0 {
			c = exec.Command(string(rbToS(pair[0]))) //nolint:gosec // running the program's command is the point
		}
	default:
		c = rbCommand(string(rbToS(first)), rest)
	}
	if argv0 != "" {
		c.Args[0] = argv0
	}
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	clean := false
	if opts != nil {
		for _, k := range opts.keys {
			v := rbUnbox(opts.vals[k])
			switch string(k) {
			case "in":
				c.Stdin = redirect(v, os.O_RDONLY, true)
			case "out":
				c.Stdout = redirect(v, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, false)
			case "err":
				c.Stderr = redirect(v, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, false)
			case "chdir":
				c.Dir = string(rbToS(v))
			case "pgroup":
				if v != nil && v != Boolean(false) {
					c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
				}
			case "unsetenv_others", "close_others":
				if _, ok := v.(Boolean); !ok && v != nil {
					panic(NewArgumentError(Ref(String("expected true or false as " + string(k) + ": " + string(rbInspect(v))))))
				}
				if k == "unsetenv_others" {
					clean = rbTruthy(v)
				}
			}
		}
	}
	if env != nil || clean {
		base := os.Environ()
		if clean {
			base = nil
		}
		vars := map[string]string{}
		var order []string
		for _, kv := range base {
			k, v, _ := strings.Cut(kv, "=")
			if _, seen := vars[k]; !seen {
				order = append(order, k)
			}
			vars[k] = v
		}
		for k, v := range env {
			if v == nil {
				delete(vars, k)
				continue
			}
			if _, seen := vars[k]; !seen {
				order = append(order, k)
			}
			vars[k] = string(rbToS(v))
		}
		c.Env = nil
		for _, k := range order {
			if v, ok := vars[k]; ok {
				c.Env = append(c.Env, k+"="+v)
			}
		}
	}
	return c, closers
}

// rbSpawnFile is a spawn redirection's file: an IO's own descriptor, a path opened with flag (owned: the caller closes it), or none for :close.
func rbSpawnFile(v any, flag int, reading bool) (f *os.File, owned bool) {
	switch x := v.(type) {
	case *IO:
		if reading && x.rf != nil {
			return x.rf, false
		}
		if !reading && x.wf != nil {
			return x.wf, false
		}
		return x.rbOSFile(), false
	case String:
		f, err := os.OpenFile(string(x), flag, 0o644) //nolint:gosec // the program's own redirection
		if err != nil {
			panic(rbSysErr(err, "rb_sysopen", string(x)))
		}
		return f, true
	}
	return nil, false // :close, nil
}
