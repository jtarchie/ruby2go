//go:build rb2go_prelude

package prelude

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os/exec"
)

// rbOpen3Cmd execs argv directly (no shell), matching Open3's array form.
func rbOpen3Cmd(name string, rest []String) *exec.Cmd {
	argv := make([]string, len(rest))
	for i, a := range rest {
		argv[i] = string(a)
	}
	return exec.Command(name, argv...) //nolint:gosec // caller-directed command, same contract as MRI's Open3
}

// rbExecErr maps a failed-to-start command to MRI's Errno message shape ("<strerror> - <cmd>", unlike rbSysErr's File errors).
func rbExecErr(err error, name string) any {
	switch {
	case errors.Is(err, exec.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return NewErrno_ENOENT(Ref(String("No such file or directory - " + name)))
	case errors.Is(err, fs.ErrPermission):
		return NewErrno_EACCES(Ref(String("Permission denied - " + name)))
	}
	return NewIOError(Ref(String(err.Error())))
}

// rbOpen3Status reads a finished command's exit info into a Process::Status.
func rbOpen3Status(c *exec.Cmd) *Process_Status {
	ps := c.ProcessState
	return &Process_Status{pid: ps.Pid(), code: ps.ExitCode()}
}

// rbOpen3Run raises Errno only on a start failure; a nonzero exit is a normal Process::Status.
func rbOpen3Run(c *exec.Cmd, name string) *Process_Status {
	err := c.Run()
	if c.ProcessState == nil {
		panic(rbExecErr(err, name))
	}
	return rbOpen3Status(c)
}

// rbOpen3Capture2 is Open3.capture2.
func rbOpen3Capture2(name string, rest []String) (String, *Process_Status) {
	c := rbOpen3Cmd(name, rest)
	var out bytes.Buffer
	c.Stdout = &out
	st := rbOpen3Run(c, name)
	return String(out.String()), st
}

// rbOpen3Capture2e is Open3.capture2e; CombinedOutput uses one pipe when Stdout and Stderr match, so order matches MRI's dup2.
func rbOpen3Capture2e(name string, rest []String) (String, *Process_Status) {
	c := rbOpen3Cmd(name, rest)
	b, err := c.CombinedOutput()
	if c.ProcessState == nil {
		panic(rbExecErr(err, name))
	}
	return String(b), rbOpen3Status(c)
}

// rbOpen3Capture3 is Open3.capture3.
func rbOpen3Capture3(name string, rest []String) (String, String, *Process_Status) {
	c := rbOpen3Cmd(name, rest)
	var out, errOut bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errOut
	st := rbOpen3Run(c, name)
	return String(out.String()), String(errOut.String()), st
}

// rbOpen3CloseW closes an Open3::Writer's pipe once.
func rbOpen3CloseW(w *Open3_Writer) {
	if w.closed {
		return
	}
	w.closed = true
	_ = w.w.Close()
}

// rbOpen3CloseR closes an Open3::Reader's pipe once.
func rbOpen3CloseR(r *Open3_Reader) {
	if r.closed {
		return
	}
	r.closed = true
	_ = r.c.Close()
}

// rbOpen3Wait memoizes Wait: wait_thr.value may run inside the block and again from popen3's own cleanup, but a *exec.Cmd waits only once.
func rbOpen3Wait(w *Process_Waiter) *Process_Status {
	if w.status == nil {
		_ = w.cmd.Wait()
		w.status = rbOpen3Status(w.cmd)
	}
	return w.status
}

// rbOpen3Popen3Start spawns the command with piped stdin/stdout/stderr (no shell).
func rbOpen3Popen3Start(name string, rest []String) (*Open3_Writer, *Open3_Reader, *Open3_Reader, *Process_Waiter) {
	c := rbOpen3Cmd(name, rest)
	stdin, err := c.StdinPipe()
	if err != nil {
		panic(NewIOError(Ref(String(err.Error()))))
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		panic(NewIOError(Ref(String(err.Error()))))
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		panic(NewIOError(Ref(String(err.Error()))))
	}
	if err := c.Start(); err != nil {
		panic(rbExecErr(err, name))
	}
	w := &Open3_Writer{w: stdin}
	o := &Open3_Reader{r: bufio.NewReader(stdout), c: stdout}
	e := &Open3_Reader{r: bufio.NewReader(stderr), c: stderr}
	wt := &Process_Waiter{cmd: c}
	return w, o, e, wt
}

// rbOpen3Popen3Close is popen3's ensure: close the pipes (a no-op for ones the block already closed) and reap the process.
func rbOpen3Popen3Close(w *Open3_Writer, o, e *Open3_Reader, wt *Process_Waiter) {
	rbOpen3CloseW(w)
	rbOpen3CloseR(o)
	rbOpen3CloseR(e)
	rbOpen3Wait(wt)
}
