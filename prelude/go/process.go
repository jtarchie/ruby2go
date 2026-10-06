//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbProcessStart anchors the monotonic clock; Go's time.Since reads the monotonic reading it carries.
var rbProcessStart = time.Now()

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
func rbCommand(cmd string, args []String) *exec.Cmd {
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
	st := &Process_Status{code: 0}
	if c.Process != nil {
		st.pid = c.Process.Pid
	}
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		st.code = ee.ExitCode()
	default:
		st.code = 127
		rbLastStatus.Store(st)
		return false
	}
	rbLastStatus.Store(st)
	return true
}

func rbSystem(cmd string, args []String) *Boolean {
	c := rbCommand(cmd, args)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if !rbRunCommand(c) {
		return nil
	}
	return Ref(Boolean(rbLastStatus.Load().code == 0))
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
	rbFlush()
	if err := c.Start(); err != nil {
		panic(NewErrno_ENOENT(Ref(String("No such file or directory - " + cmd))))
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
	st := &Process_Status{pid: pid}
	var ee *exec.ExitError
	if errors.As(ch.err, &ee) {
		st.code = ee.ExitCode()
	}
	rbLastStatus.Store(st)
	return Integer(pid)
}

// rbExec replaces the process with the command, as MRI's exec; stdout is flushed first.
func rbExec(cmd string, args []String) {
	c := rbCommand(cmd, args)
	if c.Err != nil {
		panic(NewErrno_ENOENT(Ref(String("No such file or directory - " + cmd))))
	}
	rbFlush()
	if err := syscall.Exec(c.Path, c.Args, os.Environ()); err != nil {
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
func rbPopen(cmd any, mode string) *IO {
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
	var c *exec.Cmd
	switch v := rbUnbox(cmd).(type) {
	case String:
		c = rbCommand(string(v), nil)
	case *Array[String]:
		if len(v.s) == 0 {
			panic(NewArgumentError(Ref(String("wrong number of arguments"))))
		}
		c = rbCommand(string(v.s[0]), v.s[1:])
	case *Array[any]:
		args := make([]String, 0, len(v.s))
		for _, a := range v.s {
			args = append(args, rbToS(a))
		}
		if len(args) == 0 {
			panic(NewArgumentError(Ref(String("wrong number of arguments"))))
		}
		c = rbCommand(string(args[0]), args[1:])
	default:
		panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(cmd) + " into String"))))
	}
	var ours, theirs []*os.File // our ends, then the child's, closed in the parent once it starts
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
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
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
		panic(NewErrno_ENOENT(Ref(String("No such file or directory - " + c.Path))))
	}
	return rbIONew(rf, wf, c)
}

// rbPopenReap waits for popen's child, its pipes already closed, and records $?.
func rbPopenReap(c *exec.Cmd) {
	err := c.Wait()
	st := &Process_Status{code: 0, pid: c.Process.Pid}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		st.code = ee.ExitCode()
	}
	rbLastStatus.Store(st)
}

// rbLastStatusOpt is $?: the last child's status, nil before any.
func rbLastStatusOpt() **Process_Status {
	if st := rbLastStatus.Load(); st != nil {
		return &st
	}
	return nil
}
