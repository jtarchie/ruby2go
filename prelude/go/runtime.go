//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

var stdout = bufio.NewWriter(os.Stdout)

// stdoutMu serializes writes: threads are goroutines, and there is no GVL.
var stdoutMu sync.Mutex

// stdoutTTY: MRI flushes a terminal's stdout on every write, so prompts and progress show at once; pipes stay buffered, as in MRI.
var stdoutTTY = func() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}()

// stdoutSync is `$stdout.sync = true`: flush every write, terminal or not.
var stdoutSync atomic.Bool

// rbWriteOut is Kernel#puts/print/p's write: to $stdout's object when one is assigned (decision 109), else stdout.
func rbWriteOut(s string) {
	if rbRedirected.Load() {
		if t := rbRedirectTarget(1); t != nil {
			t.Write(String(s))
			return
		}
	}
	if c := rbStdoutConv.Load(); c != nil { // STDOUT's own encoding, not a redirect target's
		s = (*c)(s)
	}
	rbWrite(s)
}

func rbWrite(s string) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	_, _ = stdout.WriteString(s)
	if stdoutTTY || stdoutSync.Load() {
		_ = stdout.Flush()
	}
}

// `$stdout = io` / `$stderr = io` (decision 109): Kernel's output and
// writes through `$stdout`/`$stderr` go to the object, through its
// `write`, until an IO on that fd is assigned back; STDOUT itself keeps
// writing to the real stream, as in MRI. Reading `$stdout` while one is
// assigned gives an IO bound to the object (rbStdoutIO), so `orig =
// $stdout` restores exactly it. A program that never assigns pays one
// atomic load per Kernel write.
var (
	rbRedirected atomic.Bool
	rbRedirectMu sync.RWMutex
	rbRedirects  [3]rbWriter
)

type rbWriter interface{ Write(any) Integer }

func rbRedirectTarget(fd int) rbWriter {
	rbRedirectMu.RLock()
	defer rbRedirectMu.RUnlock()
	return rbRedirects[fd]
}

func rbSetStdout(v any) any { return rbRedirect(1, v, "$stdout") }
func rbSetStderr(v any) any { return rbRedirect(2, v, "$stderr") }

func rbStdoutIO() *IO { return rbStreamIO(1, STDOUT) }
func rbStderrIO() *IO { return rbStreamIO(2, STDERR) }

func rbStreamIO(fd int, real *IO) *IO {
	if rbRedirected.Load() {
		if t := rbRedirectTarget(fd); t != nil {
			return &IO{fd: fd, via: t}
		}
	}
	return real
}

func rbRedirect(fd int, v any, name string) any {
	var w rbWriter
	switch x := rbUnbox(v).(type) {
	case *IO:
		switch {
		case x.own || x.fd != fd: // STDERR (or a pipe) as $stdout: writes go to it
			w = x
		case x.via != nil: // an earlier $stdout, bound to its object
			w = x.via
		}
	case rbWriter:
		w = x
	default:
		panic(NewTypeError(Ref(String(name + " must have write method, " + rbClassName(v) + " given"))))
	}
	rbRedirectMu.Lock()
	rbRedirects[fd] = w
	rbRedirected.Store(rbRedirects[1] != nil || rbRedirects[2] != nil)
	rbRedirectMu.Unlock()
	return v
}

// rbFlushIfTTY keeps a terminal's stdout ahead of stderr; on a pipe MRI leaves it buffered.
func rbFlushIfTTY() {
	if stdoutTTY {
		rbFlush()
	}
}

var rbStdin = bufio.NewReader(os.Stdin)

func (self *File) rbReadable() {
	if self.f == nil {
		panic(NewIOError(Ref[String]("closed stream")))
	}
	if self.r == nil {
		panic(NewIOError(Ref[String]("not opened for reading")))
	}
}

func (self *StringIO) rbReadable() {
	if !self.readOpen {
		panic(NewIOError(Ref[String]("not opened for reading")))
	}
}

func (self *StringIO) rbWritable() {
	if !self.writeOpen {
		panic(NewIOError(Ref[String]("not opened for writing")))
	}
}

func rbFlush() {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	_ = stdout.Flush()
}

// rbTrapSignals starts the signal loop. Untrapped, SIGINT/SIGTERM flush
// stdout and then kill the program by the signal (exit 130/143 in a shell),
// as MRI's default does; Kernel#trap replaces that per signal (decision 100).
// ponytail: MRI raises Interrupt/SignalException in the main thread, so rescue and ensure run; Go can't inject a panic into another goroutine.
func rbTrapSignals() {
	rbMainGoID = rbGoID()
	signal.Notify(rbSigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range rbSigCh {
			if rbTrapHook != nil && rbTrapHook(sig) {
				continue
			}
			rbInterruptPost(sig)
		}
	}()
}

// Catchable Ctrl-C (decision 60). An untrapped SIGINT/SIGTERM is posted
// here; a blocking call on the main goroutine (sleep, Thread#join/value,
// Queue#pop, ConditionVariable#wait) takes it (rbTakeInterrupt) and raises
// Interrupt/SignalException there, so rescue and ensure run as in MRI.
// Go cannot interrupt a computing goroutine, so after rbInterruptGrace
// with nothing taking it the program dies by the signal, as before.
var (
	rbMainGoID       int64
	rbInterruptMu    sync.Mutex
	rbInterruptSig   os.Signal             // pending, not yet taken
	rbInterruptCh    = make(chan struct{}) // closed while one is pending; a fresh one after it is taken
	rbInterruptTimer *time.Timer
	rbInterruptWake  sync.Map // blocked main-goroutine waiters that need a nudge (a Queue's cond), by owner → func()
)

const rbInterruptGrace = 200 * time.Millisecond

func rbInterruptPost(sig os.Signal) {
	rbInterruptMu.Lock()
	defer rbInterruptMu.Unlock()
	if rbInterruptSig != nil {
		return
	}
	rbInterruptSig = sig
	close(rbInterruptCh)
	rbInterruptTimer = time.AfterFunc(rbInterruptGrace, func() { rbDieBySignal(sig) })
	rbInterruptWake.Range(func(_, f any) bool {
		f.(func())()
		return true
	})
}

// rbInterruptC is the channel a blocking call selects on: closed while a signal is pending.
func rbInterruptC() <-chan struct{} {
	rbInterruptMu.Lock()
	defer rbInterruptMu.Unlock()
	return rbInterruptCh
}

func rbInterruptPending() bool {
	rbInterruptMu.Lock()
	defer rbInterruptMu.Unlock()
	return rbInterruptSig != nil
}

// rbTakeInterrupt raises the pending signal on the main goroutine as
// Interrupt (SIGINT) or SignalException; on another goroutine, or with
// none pending, it returns.
func rbTakeInterrupt() {
	if rbGoID() != rbMainGoID {
		return
	}
	rbInterruptMu.Lock()
	sig := rbInterruptSig
	if sig == nil {
		rbInterruptMu.Unlock()
		return
	}
	rbInterruptSig = nil
	rbInterruptTimer.Stop()
	rbInterruptCh = make(chan struct{})
	rbInterruptMu.Unlock()
	if sig == os.Interrupt {
		panic(NewInterrupt(nil))
	}
	panic(NewSignalException(Integer(sig.(syscall.Signal))))
}

// rbDieBySignal flushes and ends the program by sig's default action, as
// MRI does for an unhandled one; 128+sig if the signal is ignored (a
// background job) and does not end it.
func rbDieBySignal(sig os.Signal) {
	rbFlush()
	signal.Reset(sig)
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(sig)
	time.Sleep(100 * time.Millisecond)
	os.Exit(128 + int(sig.(syscall.Signal)))
}

// rbTrapHook consults the trap table; Kernel#trap sets it, so a program without trap carries no table (decision 49).
var rbTrapHook func(sig os.Signal) bool

// rbTrapped runs or ignores a trapped signal, reporting whether sig had a handler.
func rbTrapped(sig os.Signal) bool {
	rbTrapMu.Lock()
	h, ok := rbTraps[sig]
	rbTrapMu.Unlock()
	switch {
	case ok && h.blk != nil:
		rbRunTrap(h.blk, sig)
	case ok && h.cmd == "IGNORE":
	default:
		return false
	}
	return true
}

// rbTrap is one Kernel#trap handler: a block, or a command string.
type rbTrap struct {
	blk func(Integer)
	cmd string
}

var (
	rbSigCh  = make(chan os.Signal, 8)
	rbTrapMu sync.Mutex
	rbTraps  = map[os.Signal]rbTrap{}
)

// rbRunTrap runs a trap block on the signal goroutine (MRI: the main
// thread). An exception from it ends the program as an uncaught one.
func rbRunTrap(blk func(Integer), sig os.Signal) {
	defer func() {
		if r := recover(); r != nil {
			status, e := rbExitStatus(rbWrapPanic(r))
			rbFinish(status, e)
			os.Exit(0)
		}
	}()
	blk(Integer(sig.(syscall.Signal)))
}

var rbSignalNums = map[string]syscall.Signal{
	"HUP": syscall.SIGHUP, "INT": syscall.SIGINT, "QUIT": syscall.SIGQUIT, "ILL": syscall.SIGILL,
	"TRAP": syscall.SIGTRAP, "ABRT": syscall.SIGABRT, "IOT": syscall.SIGABRT, "BUS": syscall.SIGBUS,
	"FPE": syscall.SIGFPE, "KILL": syscall.SIGKILL, "USR1": syscall.SIGUSR1, "SEGV": syscall.SIGSEGV,
	"USR2": syscall.SIGUSR2, "PIPE": syscall.SIGPIPE, "ALRM": syscall.SIGALRM, "TERM": syscall.SIGTERM,
	"CHLD": syscall.SIGCHLD, "CONT": syscall.SIGCONT, "STOP": syscall.SIGSTOP, "TSTP": syscall.SIGTSTP,
	"TTIN": syscall.SIGTTIN, "TTOU": syscall.SIGTTOU, "URG": syscall.SIGURG, "XCPU": syscall.SIGXCPU,
	"XFSZ": syscall.SIGXFSZ, "VTALRM": syscall.SIGVTALRM, "PROF": syscall.SIGPROF, "WINCH": syscall.SIGWINCH,
	"IO": syscall.SIGIO, "SYS": syscall.SIGSYS, "EXIT": 0,
}

// rbSignalArg reads a signal as MRI's trap and Process.kill take it: a
// name with or without SIG (String or Symbol) or a number.
func rbSignalArg(v any) syscall.Signal {
	switch v := rbUnbox(v).(type) {
	case Integer:
		for _, s := range rbSignalNums {
			if int(s) == int(v) {
				return s
			}
		}
		panic(NewArgumentError(Ref(String(fmt.Sprintf("invalid signal number (%d)", v)))))
	case String, Symbol:
		name := strings.TrimPrefix(fmt.Sprint(v), "SIG")
		if s, ok := rbSignalNums[name]; ok {
			return s
		}
		panic(NewArgumentError(Ref(String("unsupported signal 'SIG" + name + "'"))))
	}
	panic(NewArgumentError(Ref(String("bad signal type " + rbClassName(v)))))
}

// rbSetTrap is Kernel#trap: it installs blk or cmd ("IGNORE", "DEFAULT",
// "SYSTEM_DEFAULT", "EXIT" or "") and returns the previous command, nil
// when that was a block (MRI returns the Proc).
func rbSetTrap(v any, blk func(Integer), cmd string) *String {
	sig := rbSignalArg(v)
	switch {
	case sig == syscall.SIGKILL || sig == syscall.SIGSTOP:
		panic(NewErrno_EINVAL(Ref(String("Invalid argument - SIG" + rbSignalName(sig)))))
	case slices.Contains([]syscall.Signal{syscall.SIGSEGV, syscall.SIGBUS, syscall.SIGILL, syscall.SIGFPE, syscall.SIGVTALRM}, sig):
		panic(NewArgumentError(Ref(String("can't trap reserved signal: SIG" + rbSignalName(sig)))))
	case sig == 0: // "EXIT": an at_exit handler
		if blk != nil {
			rbAtExitPush(func() { blk(0) })
		}
		return nil
	}
	if cmd == "SYSTEM_DEFAULT" {
		cmd = "DEFAULT"
	}
	rbTrapMu.Lock()
	defer rbTrapMu.Unlock()
	rbTrapHook = rbTrapped
	old, had := rbTraps[sig]
	rbTraps[sig] = rbTrap{blk: blk, cmd: cmd}
	switch {
	case blk != nil || cmd == "IGNORE":
		signal.Notify(rbSigCh, sig)
	case sig != syscall.SIGINT && sig != syscall.SIGTERM:
		signal.Reset(sig)
	}
	switch {
	case !had:
		return Ref(String("DEFAULT"))
	case old.blk != nil:
		return nil
	}
	return Ref(String(old.cmd))
}

func rbSignalName(sig syscall.Signal) string {
	for n, s := range rbSignalNums {
		if s == sig && n != "IOT" {
			return n
		}
	}
	return strconv.Itoa(int(sig))
}

// rbAtExit holds Kernel#at_exit handlers; rbTopRecover pops them LIFO, so
// one registered while handlers run is next.
var (
	rbAtExitMu sync.Mutex
	rbAtExit   []func()
)

func rbAtExitPush(f func()) {
	rbAtExitMu.Lock()
	defer rbAtExitMu.Unlock()
	rbAtExit = append(rbAtExit, f)
}

func rbAtExitPop() func() {
	rbAtExitMu.Lock()
	defer rbAtExitMu.Unlock()
	if len(rbAtExit) == 0 {
		return nil
	}
	f := rbAtExit[len(rbAtExit)-1]
	rbAtExit = rbAtExit[:len(rbAtExit)-1]
	return f
}

// rbTopRecover ends the program as MRI does: at_exit handlers run first;
// then the uncaught exception, if any, prints `msg (Class)`. The status is
// 1 after an uncaught exception, SystemExit's status after exit, and a
// handler's exit or exception replaces it. Output is flushed first so
// partial output before a crash matches.
func rbTopRecover() {
	status, main := 0, any(nil)
	if r := recover(); r != nil {
		status, main = rbExitStatus(r)
	}
	rbFinish(status, main)
}

// rbFinish runs the at_exit handlers, prints main (an uncaught exception
// or nil) and exits with the resulting status; it returns only for 0.
func rbFinish(status int, main any) {
	for f := rbAtExitPop(); f != nil; f = rbAtExitPop() {
		rbExitStatusNow.Store(int64(status))
		func() {
			defer func() {
				if r := recover(); r != nil {
					var err any
					if status, err = rbExitStatus(r); err != nil {
						rbFlushIfTTY()
						rbPrintUncaught(err)
					}
				}
			}()
			f()
		}()
	}
	rbFlush()
	if main != nil {
		rbPrintUncaught(main)
		if se, ok := main.(interface{ Signo() Integer }); ok { // an uncaught Interrupt ends the program by its signal, as in MRI
			rbDieBySignal(syscall.Signal(se.Signo()))
		}
	}
	if status != 0 {
		os.Exit(status)
	}
}

// rbExitStatusNow is the status the program would exit with, for at_exit handlers.
var rbExitStatusNow atomic.Int64

// rbIsRef reports whether a's address is its identity: its class's Go type is a pointer (rbClassRefs, decision 89).
func rbIsRef(a any) bool {
	v, ok := a.(interface{ _ClassID() int })
	return ok && rbClassRefs[v._ClassID()]
}

// rbObjectID is Kernel#object_id: a pointer's address, else the value's hash.
func rbObjectID(a any) Integer {
	if rbIsRef(a) {
		return Integer(maphash.Comparable(rbHashSeed, a) >> 2) //nolint:gosec // an id, not arithmetic
	}
	return rbHash(a) & (1<<62 - 1)
}

// rbExitStatus is the exit status r ends the program with, and r itself
// when it is an error to print rather than a SystemExit.
func rbExitStatus(r any) (int, any) {
	if e, ok := r.(SystemExitI); ok {
		return int(e.Status()), nil
	}
	return 1, r
}

func rbPrintUncaught(r any) {
	if e, ok := r.(ExceptionI); ok {
		fmt.Fprintf(os.Stderr, "%s (%s)\n", e.Message(), rbClassName(r))
	} else {
		fmt.Fprintf(os.Stderr, "%v\n", r)
	}
}

// rbArg is a dispatcher's args[i], once rbArity has checked the count (dynArg).
func rbArg(args []any, i int) any { return args[i] }

// Ref boxes a value into T? (represented as *T).
func Ref[T any](v T) *T { return &v }

// rbZero fills a left-out argument; the callee sees rbArgc and runs its own default.
func rbZero[T any]() (z T) { return z }

// Opt converts T? to untyped: a nil *T must become an untyped nil or a
// `case nil` type switch misses it.
func Opt[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// rbFlat collapses a generic E? instantiated with E = T? (a **T) to T?:
// Ruby has one nil.
func rbFlat[T any](p **T) *T {
	if p == nil {
		return nil
	}
	return *p
}

// OptOf converts untyped to T?; want names T for rbAs's TypeError.
func OptOf[T any](a any, want string) *T {
	if a == nil {
		return nil
	}
	v := rbAs[T](a, want)
	return &v
}

// rbAs converts untyped to T, raising MRI's TypeError when a is not one:
// a bare a.(T) would panic as a Go error, a StandardError.
func rbAs[T any](a any, want string) T {
	v, ok := a.(T)
	if !ok {
		return rbAsSlow[T](a, want)
	}
	return v
}

// rbAsSlow converts as rbConv does (nil for untyped, other Array/Hash
// instantiations, an Array into a tuple), else raises.
func rbAsSlow[T any](a any, want string) T {
	if v, ok := rbConv[T](a); ok {
		return v
	}
	panic(rbConvError(a, want))
}

// rbConvError is MRI's "no implicit conversion" TypeError.
func rbConvError(a any, want string) any {
	name := rbClassName(a)
	switch r := a.(type) {
	case nil:
		name = "nil"
	case Boolean:
		name = strconv.FormatBool(bool(r))
	case rbModule:
		name = strings.ToUpper(r._Kind()[:1]) + r._Kind()[1:]
	}
	return NewTypeError(Ref(String("no implicit conversion of " + name + " into " + want)))
}

type I_ToS interface{ ToS() String }
type I_Inspect interface{ Inspect() String }

// rbToS, rbInspect, rbEq and rbCmp take E = T? values from generic
// code as the box itself; rbUnbox (generated) opens it.
func rbToS(a any) String {
	a = rbUnbox(a)
	if a == nil {
		return ""
	}
	if s, ok := a.(I_ToS); ok {
		return s.ToS()
	}
	return rbObjToS(a)
}

// rbObjToS is Kernel#to_s; only heap objects have an address to show.
func rbObjToS(a any) String {
	s := "#<" + rbClassName(a)
	if rbIsRef(a) {
		addr := strings.TrimPrefix(fmt.Sprintf("%p", a), "0x") // %p is a pointer's address
		s += ":0x" + strings.Repeat("0", max(16-len(addr), 0)) + addr
	}
	return String(s + ">")
}

// rbIvar feeds Kernel#inspect; !opt means nil was never assigned, which MRI doesn't list.
type rbIvar struct {
	name  string
	val   any
	opt   bool
	isNil bool // a nil pointer or interface, decided from the field's type when _Ivars is generated
}

// rbObjInspect is Kernel#inspect; rbInspectEnter gives MRI's "..." for an object that holds itself.
func rbObjInspect(a any) String {
	o, ok := a.(interface{ _Ivars() []rbIvar })
	if !ok {
		return rbObjToS(a)
	}
	s := string(rbObjToS(a))
	var b strings.Builder
	b.WriteString(s[:len(s)-1])
	if !rbInspectEnter(a) {
		return String(b.String() + " ...>")
	}
	defer rbInspectLeave(a)
	sep := " "
	for _, iv := range o._Ivars() {
		if !iv.opt && (iv.val == nil || iv.isNil) {
			continue
		}
		b.WriteString(sep + iv.name + "=")
		sep = ", "
		if _, ok := iv.val.(I_Inspect); ok || iv.val == nil {
			b.WriteString(string(rbInspect(iv.val)))
		} else { // a Go value behind a prelude ivar
			b.WriteString(string(rbObjToS(iv.val)))
		}
	}
	return String(b.String() + ">")
}

func rbInspect(a any) String {
	a = rbUnbox(a)
	if a == nil {
		return "nil"
	}
	return a.(I_Inspect).Inspect()
}

// rbInspecting holds the containers whose inspect is on the stack, so one
// that holds itself prints [...] / {...} like MRI instead of overflowing.
// ponytail: one set for all threads (MRI's is per-thread), so two threads
// inspecting the same container at once may see [...]; a goroutine-local
// set needs a goroutine id Go does not expose.
var (
	rbInspectingMu sync.Mutex
	rbInspecting   = map[any]struct{}{}
)

// rbInspectEnter marks p as being inspected; false means it already is.
// A true result must be paired with a deferred rbInspectLeave(p).
func rbInspectEnter(p any) bool {
	rbInspectingMu.Lock()
	defer rbInspectingMu.Unlock()
	if _, ok := rbInspecting[p]; ok {
		return false
	}
	rbInspecting[p] = struct{}{}
	return true
}

func rbInspectLeave(p any) {
	rbInspectingMu.Lock()
	defer rbInspectingMu.Unlock()
	delete(rbInspecting, p)
}

func rbTruthy(a any) bool {
	switch v := a.(type) {
	case nil:
		return false
	case Boolean:
		return bool(v)
	}
	return true
}

// rbTruthyOpt tests a Boolean?: false is as falsy as nil.
func rbTruthyOpt(p *Boolean) bool { return p != nil && bool(*p) }

func rbEq[T comparable](a, b T) Boolean {
	x, y := rbUnbox(any(a)), rbUnbox(any(b))
	if e, ok := x.(interface{ Op_eq(any) Boolean }); ok {
		return e.Op_eq(y)
	}
	if e, ok := x.(interface{ _EqAny(any) Boolean }); ok {
		return e._EqAny(y) // a == typed on its argument; see emitEqAdapter
	}
	return Boolean(x == y)
}

// Hash keys, uniq and tally match by Ruby's eql?/hash. For most keys that
// is Go ==: strings, numbers, symbols, and objects by identity. Arrays,
// hashes, Regexps, Structs and anything else defining both eql? and hash
// match by value, and so does a T? box (*T) by what it points at.

// rbPlainKey reports whether K's Go == is eql?, so a map can key by K.
func rbPlainKey[K comparable]() bool {
	var z K
	switch z := any(z).(type) {
	case String, Symbol, Integer, Float, Boolean:
		return true
	case interface{ rbPlain() bool }:
		return z.rbPlain()
	}
	return false
}

type rbEqlHash interface {
	EqlQ(other any) Boolean
	Hash() Integer
}

var rbHashSeed = maphash.MakeSeed()

// rbValueKey is k's hash when k matches by value rather than Go ==.
func rbValueKey(k any) (uint64, bool) {
	if p, ok := rbKeyUnbox(k); ok {
		return rbKeyHash(p), true
	}
	if v, ok := k.(rbEqlHash); ok {
		return uint64(v.Hash()), true
	}
	return 0, false
}

func rbKeyHash(k any) uint64 {
	if h, ok := rbValueKey(k); ok {
		return h
	}
	return maphash.Comparable(rbHashSeed, k)
}

func rbKeyEql(a, b any) bool {
	if p, ok := rbKeyUnbox(a); ok {
		a = p
	}
	if p, ok := rbKeyUnbox(b); ok {
		b = p
	}
	if a == b {
		return true
	}
	if e, ok := a.(rbEqlHash); ok {
		return bool(e.EqlQ(b))
	}
	return false
}

// rbHash is #hash on an untyped value.
func rbHash(a any) Integer {
	if h, ok := a.(interface{ Hash() Integer }); ok {
		return h.Hash()
	}
	return Integer(rbKeyHash(a))
}

// rbCmp is <=> for sort, min and max. Typed values have Op_cmp(T);
// T? boxes compare their values (rbCmpBox); untyped ones (T is any) go
// through the generated DynOp_cmp wrappers (rbCmpFailed), which answer
// nil for an incomparable argument, as MRI's <=> does.
func rbCmp[T comparable](a, b T) Integer {
	if c, ok := any(a).(interface{ Op_cmp(T) Integer }); ok {
		return c.Op_cmp(b)
	}
	return rbCmpBox(any(a), any(b))
}

// rbCmpOpt compares the values in two T? boxes (see rbCmpBox).
func rbCmpOpt[T comparable](a, b *T) Integer {
	if a == nil || b == nil {
		return rbCmpFailed(a, b)
	}
	return rbCmp(*a, *b)
}

// rbCmpFailed is <=> where no Op_cmp(T) applies: an untyped value asks
// its DynOp_cmp wrapper, and a nil answer raises MRI's rb_cmperr.
func rbCmpFailed(a, b any) Integer {
	a, b = rbUnbox(a), rbUnbox(b)
	if c, ok := a.(interface{ DynOp_cmp(...any) any }); ok {
		if r, ok := c.DynOp_cmp(b).(Integer); ok {
			return r
		}
	}
	panic(rbCmpErr(a, b))
}

// rbCmpErr is MRI's rb_cmperr, raised where <=> answered nil: it
// inspects immediates and Floats and names the class of anything else.
func rbCmpErr(a, b any) *ArgumentError {
	with := rbClassName(b)
	switch b.(type) {
	case nil, Boolean, Integer, Float, Symbol:
		with = string(rbInspect(b))
	}
	return NewArgumentError(Ref(String("comparison of " + rbClassName(a) + " with " + with + " failed")))
}

func rbIdentical(a, b any) bool {
	if as, ok := a.(String); ok {
		bs, ok := b.(String)
		// Identity of immutable strings is their backing pointer.
		return ok && len(as) == len(bs) && unsafe.StringData(string(as)) == unsafe.StringData(string(bs)) //nolint:gosec // pointer compare only
	}
	return a == b
}

// rbNewStr is a String method's result, a new object in MRI: when it is
// the receiver's own bytes (nothing to strip, replace or pad), it is
// copied, so equal? and frozen? don't take it for the receiver. Only that
// case pays for the copy.
// ponytail: other shared bytes still look identical (equal slices of one string, strconv's small-number table, a "" Go boxes to zeroVal); a boxed String would fix them.
func rbNewStr(recv, s String) String {
	if len(s) == len(recv) && unsafe.StringData(string(s)) == unsafe.StringData(string(recv)) { //nolint:gosec // pointer compare only
		return rbStrClone(s)
	}
	return s
}

// rbStrClone copies s to bytes of its own; "" gets an address of its own.
func rbStrClone(s String) String {
	if s == "" {
		return String(unsafe.String(new(byte), 0)) //nolint:gosec // an address for identity only
	}
	return String(strings.Clone(string(s)))
}

// rbStrID is a String's identity: backing pointer and length, as in rbIdentical.
type rbStrID struct {
	p *byte
	n int
}

var (
	rbFrozenMu   sync.Mutex
	rbFrozenStrs map[rbStrID]bool // the literals (seeded on first use) and every String#freeze receiver
)

// rbStrFrozen reports whether s is a literal or was frozen, and with
// freeze also marks it. Strings built at run time have fresh backing
// arrays, so they aren't in the set, like MRI's unfrozen strings.
// ponytail: frozen computed strings stay reachable from the set; use weak pointers if that leak shows up.
func rbStrFrozen(s string, freeze bool) bool {
	id := rbStrID{unsafe.StringData(s), len(s)} //nolint:gosec // identity only
	rbFrozenMu.Lock()
	defer rbFrozenMu.Unlock()
	if rbFrozenStrs == nil {
		rbFrozenStrs = make(map[rbStrID]bool, len(rbStringLits))
		for _, l := range rbStringLits {
			rbFrozenStrs[rbStrID{unsafe.StringData(l), len(l)}] = true //nolint:gosec // identity only
		}
	}
	was := rbFrozenStrs[id]
	if freeze {
		rbFrozenStrs[id] = true
	}
	return was
}

func rbIsA[I any](r any) bool {
	_, ok := r.(I)
	return ok
}

// rbSplat converts a splatted array's elements to a rest param's type.
func rbSplat[T, E any](s []T, conv func(T) E) []E {
	out := make([]E, len(s))
	for i, v := range s {
		out[i] = conv(v)
	}
	return out
}

func rbClassName(a any) string {
	if id := rbClassID(a); id >= 0 {
		return rbClassNames[id]
	}
	return "Object" // a Go value behind a prelude ivar
}

// rbClassID is the generated ID of a's class (decision 82): nil and procs
// by value, a T? box by what it holds, anything else by its _ClassID.
func rbClassID(a any) int {
	switch v := rbUnbox(a).(type) {
	case nil:
		return rbNilClassID
	case interface{ _ClassID() int }:
		return v._ClassID()
	}
	if rbIsProc(a) {
		return rbProcClassID
	}
	return -1
}

// rbKindOf is is_a?(target) by ID: target is among a's class's ancestors.
func rbKindOf(a any, target int) bool {
	id := rbClassID(a)
	return id >= 0 && slices.Contains(rbAncestry[id], target)
}

// rbWrapPanic converts Go runtime panics into Ruby exceptions so a
// catch-all rescue sees a StandardError.
func rbWrapPanic(r any) any {
	switch e := r.(type) {
	case ExceptionI:
		e._Exception().__seen = true // a later capture knows this is a re-raise (decision 106)
		return r
	case rbStop, rbThrow:
		return r
	}
	if err, ok := r.(runtime.Error); ok {
		msg := err.Error()
		if strings.Contains(msg, "divide by zero") {
			return NewZeroDivisionError(Ref[String]("divided by 0"))
		}
		if strings.Contains(msg, "nil pointer") {
			return NewNoMethodError(Ref[String]("undefined method for nil"))
		}
		if strings.Contains(msg, "index out of range") {
			return NewIndexError(Ref[String](String(msg)))
		}
		return NewStandardError(Ref[String](String(msg)))
	}
	return NewStandardError(Ref[String](String(fmt.Sprint(r))))
}

// rbThrow unwinds from Kernel#throw to its Kernel#catch. Like rbStop it
// is not an exception: ensure runs, rescue passes it on.
type rbThrow struct{ tag, val any }

// rbCatchTags are the tags of the running catch blocks, so a throw with no
// match raises UncaughtThrowError where it is, as in MRI.
// ponytail: one list for all threads (MRI's is per-thread), so a thread
// may throw to another's tag and die with the throw uncaught; a
// goroutine-local list needs a goroutine id Go does not expose.
var (
	rbCatchMu   sync.Mutex
	rbCatchTags []any
)

func rbCatch(tag any, blk func(any) any) (res any) {
	rbCatchMu.Lock()
	rbCatchTags = append(rbCatchTags, tag)
	rbCatchMu.Unlock()
	defer func() {
		rbCatchMu.Lock()
		for i := len(rbCatchTags) - 1; i >= 0; i-- {
			if rbIdentical(rbCatchTags[i], tag) {
				rbCatchTags = slices.Delete(rbCatchTags, i, i+1)
				break
			}
		}
		rbCatchMu.Unlock()
		if r := recover(); r != nil {
			if t, ok := r.(rbThrow); ok && rbIdentical(t.tag, tag) {
				res = t.val
				return
			}
			panic(r)
		}
	}()
	return blk(tag)
}

func rbThrowTag(tag, val any) {
	rbCatchMu.Lock()
	caught := slices.ContainsFunc(rbCatchTags, func(t any) bool { return rbIdentical(t, tag) })
	rbCatchMu.Unlock()
	if !caught {
		panic(NewUncaughtThrowError("uncaught throw "+rbInspect(tag), tag, val))
	}
	panic(rbThrow{tag, val})
}

// rbStop unwinds a closure-taking each when the loop over its rbSeq
// adapter stops early, like MRI's break: ensure runs, rescue passes it on.
type rbStop struct{}

// rbSeq adapts a closure-taking each to the iter.Seq its callers range
// over (decision 4). A loop body's own exception still unwinds through
// each; if each rescues it, Go aborts, since a range function may not
// recover one.
func rbSeq[E any](each func(func(E))) iter.Seq[E] {
	return func(yield func(E) bool) {
		defer rbStopped()
		stopped := false
		each(func(x E) {
			if stopped || !yield(x) {
				stopped = true
				panic(rbStop{})
			}
		})
	}
}

func rbSeq2[K, V any](each func(func(K, V))) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		defer rbStopped()
		stopped := false
		each(func(k K, v V) {
			if stopped || !yield(k, v) {
				stopped = true
				panic(rbStop{})
			}
		})
	}
}

func rbStopped() {
	if r := recover(); r != nil {
		if _, ok := r.(rbStop); !ok {
			panic(r)
		}
	}
}

func NewHash[K, V comparable]() *Hash[K, V] {
	return &Hash[K, V]{vals: map[K]V{}, idx: rbKeyIndex[K]{plain: rbPlainKey[K]()}}
}

// `when Array` / `when Hash` in a type switch can't match a generic
// instantiation; every instantiation implements these instead.
type Array_Any interface{ _ToAny() *Array[any] }
type Hash_Any interface{ _ToAny() *Hash[any, any] }

// rbFloatToI converts like MRI: a non-finite Float raises FloatDomainError.
func rbFloatToI(f float64) Integer {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		panic(NewFloatDomainError(Ref(rbFloatToS(f))))
	}
	if f < -(1<<63) || f >= 1<<63 {
		panic(NewRangeError(Ref("float " + rbFloatToS(f) + " out of range of integer")))
	}
	return Integer(f)
}

// rbFloatPow is Float#**. MRI calls C's pow(), which glibc and macOS round
// correctly bar inputs a hair from a rounding midpoint; Go's math.Pow is
// often an ulp or more off (8.0 ** (1.0/3) is 1.9999999999999998, MRI
// 2.0). This evaluates exp(y*log|x|) in double-double (error under 2^-84)
// and rounds once. math.Pow keeps the special values, which it treats as
// C99's pow does, and results past the normal range.
// ponytail: subnormal results can be an ulp off; round them at 2^-1074 by adding 1.0 first (musl's exp specialcase) if that matters.
// ponytail: costs 5-10x math.Pow; square-and-multiply in double-double would make small integer exponents cheap.
func rbFloatPow(x, y float64) float64 {
	const ln2lo = 0x1.abc9e3b39803fp-56 // math.Ln2 - float64(math.Ln2)
	switch {
	case y == 2: // MRI's own shortcut; one rounding already
		return x * x
	case x == 0 || y == 0 || x == 1 || x == -1 || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0),
		x < 0 && y != math.Trunc(y):
		return math.Pow(x, y)
	}
	// log|x| = e*ln2 + 2s*sum(t^n/(2n+1)), with s = (f-1)/(f+1), t = s*s < 0.03
	// for f in [sqrt(1/2), sqrt(2)); f-1 is exact.
	f, e := math.Frexp(math.Abs(x))
	if f < math.Sqrt2/2 {
		f, e = 2*f, e-1
	}
	dh, dl := rbTwoSum(f, 1)
	sh := (f - 1) / dh
	sl := (math.FMA(-sh, dh, f-1) - sh*dl) / dh
	th, tl := rbDDMul(sh, sl, sh, sl)
	ah, al := 0.0, 0.0
	for n := 18; n >= 0; n-- {
		c := rbPowInv[n]
		if n > 6 { // terms under 2^-39 need no low word
			ah = ah*th + c[0]
			continue
		}
		ah, al = rbDDMul(ah, al, th, tl)
		ah, al = rbDDAdd(ah, al, c[0], c[1])
	}
	lh, ll := rbDDMul(sh, sl, 2*ah, 2*al)
	fe := float64(e)
	kh := float64(fe * math.Ln2)
	lh, ll = rbDDAdd(kh, math.FMA(fe, math.Ln2, -kh)+fe*ln2lo, lh, ll)
	// x**y = 2^k * exp(r), r = y*log|x| - k*ln2, |r| <= ln2/2
	ph := float64(y * lh)
	pl := math.FMA(y, lh, -ph) + y*ll
	if !(ph > -708.39 && ph < 709.79) { // subnormal or overflowing
		return math.Pow(x, y)
	}
	k := math.Round(ph * math.Log2E)
	kh = float64(k * math.Ln2)
	rh, rl := rbDDAdd(ph, pl, -kh, -math.FMA(k, math.Ln2, -kh)-k*ln2lo)
	// expm1(r/4) by Taylor, then expm1(2a) = expm1(a)*(expm1(a)+2) twice.
	rh, rl = rh/4, rl/4
	eh, el := 0.0, 0.0
	for n := 16; n >= 1; n-- {
		c := rbPowFact[n]
		if n > 7 { // terms under 2^-39 of the first
			eh = (eh + c[0]) * rh
			continue
		}
		eh, el = rbDDAdd(eh, el, c[0], c[1])
		eh, el = rbDDMul(eh, el, rh, rl)
	}
	for range 2 {
		mh, ml := rbDDAdd(eh, el, 2, 0)
		eh, el = rbDDMul(eh, el, mh, ml)
	}
	res, _ := rbDDAdd(1, 0, eh, el)
	res = math.Ldexp(res, int(k))
	if x < 0 && math.Mod(y, 2) != 0 {
		return -res
	}
	return res
}

// rbPowInv[n] is 1/(2n+1) and rbPowFact[n] is 1/n!, as double-doubles.
var rbPowInv, rbPowFact = func() (inv, fact [19][2]float64) {
	nf := 1.0
	for n := range inv {
		d := float64(2*n + 1)
		if n > 0 {
			nf *= float64(n)
		}
		inv[n] = [2]float64{1 / d, math.FMA(-1/d, d, 1) / d}
		fact[n] = [2]float64{1 / nf, math.FMA(-1/nf, nf, 1) / nf}
	}
	return inv, fact
}()

// Double-double arithmetic: a value is hi+lo with |lo| <= ulp(hi)/2. The
// float64() around a product stops Go fusing it into a later add (an FMA),
// which would break the error-free split.
func rbTwoSum(a, b float64) (float64, float64) {
	s := a + b
	bb := s - a
	return s, (a - (s - bb)) + (b - bb)
}

func rbDDAdd(ah, al, bh, bl float64) (float64, float64) {
	s, e := rbTwoSum(ah, bh)
	e += al + bl
	h := s + e
	return h, e - (h - s)
}

func rbDDMul(ah, al, bh, bl float64) (float64, float64) {
	p := float64(ah * bh)
	e := math.FMA(ah, bh, -p) + (ah*bl + al*bh)
	h := p + e
	return h, e - (h - p)
}

// rbIntOverflow raises where MRI would promote to a Bignum: Integer is a
// Go int (decision 35). Out of line so the checked operators still inline.
//
//go:noinline
func rbIntOverflow(a Integer, op string, b Integer) {
	panic(NewRangeError(Ref(String(fmt.Sprintf("%d %s %d overflows Integer (64-bit; no Bignum)", a, op, b)))))
}

// rbIntMul is Integer#* past its 32-bit fast path, out of line too.
//
//go:noinline
func rbIntMul(a, b Integer) Integer {
	r, ok := rbIntMulOk(a, b)
	if !ok {
		rbIntOverflow(a, "*", b)
	}
	return r
}

// rbIntMulOk is a*b and whether it fits: the signed high word of the
// 128-bit product must be the low word's sign extension.
func rbIntMulOk(a, b Integer) (Integer, bool) {
	hi, lo := bits.Mul64(uint64(a), uint64(b))
	if a < 0 {
		hi -= uint64(b)
	}
	if b < 0 {
		hi -= uint64(a)
	}
	return Integer(lo), int64(hi) == int64(lo)>>63
}

func rbFloatToS(f float64) String {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	// MRI's flo_to_s: exponent form when the decimal point sits more than
	// DBL_DIG (15) digits in and the shortest digits have no fraction. In
	// [1e15, 1e16) the shortest digits lack a fraction iff f is integral.
	abs := math.Abs(f)
	var s string
	if abs != 0 && (abs >= 1e16 || abs < 1e-4 || abs >= 1e15 && f == math.Trunc(f)) {
		s = strconv.FormatFloat(f, 'e', -1, 64)
		mant, exp, _ := strings.Cut(s, "e")
		if !strings.Contains(mant, ".") {
			mant += ".0"
		}
		return String(mant + "e" + exp)
	}
	s = strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.ContainsAny(s, ".") {
		s += ".0"
	}
	return String(s)
}

// rbCallerLoc is the "file:line" of the nearest user-code frame (a Ruby
// body outside the prelude), as MRI shows where a Thread or Ractor was made.
func rbCallerLoc() string { return rbCallerLocN(0) }

// rbCallerLocN skips n user-code frames first (warn's uplevel:); "" past the stack.
func rbCallerLocN(n int) string {
	pcs := make([]uintptr, 64)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	_, self, _, _ := runtime.Caller(0)
	mod := strings.TrimSuffix(self, "prelude/go/runtime.go")
	for {
		f, more := frames.Next()
		file := strings.TrimPrefix(f.File, mod)
		if strings.HasSuffix(file, ".rb") && !strings.HasPrefix(file, "prelude/") && !strings.Contains(f.Function, ".(") {
			if n == 0 {
				return file + ":" + strconv.Itoa(f.Line)
			}
			n--
		}
		if !more {
			return ""
		}
	}
}

// rbStringInspect is String#inspect for a UTF-8 string: controls are \u00XX, as MRI prints them for a UTF-8 string.
func rbStringInspect(s string) String { return rbStringInspectAs(s, true) }

// rbStringInspectAs inspects s; utf8 false gives the US-ASCII form of a control (\xXX), which Symbol#inspect uses.
func rbStringInspectAs(s string, utf8Controls bool) String {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n == 1 {
			fmt.Fprintf(&b, `\x%02X`, s[i])
			i++
			continue
		}
		i += n
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\f':
			b.WriteString(`\f`)
		case '\v':
			b.WriteString(`\v`)
		case '\a':
			b.WriteString(`\a`)
		case '\b':
			b.WriteString(`\b`)
		case 0x1b:
			b.WriteString(`\e`)
		case '#':
			b.WriteByte('#')
		default:
			switch {
			case (r < 0x20 || r == 0x7f) && utf8Controls:
				// ponytail: strings carry no encoding, so every String gets the
				// UTF-8 form; MRI prints a US-ASCII string's controls (Integer#chr) as \x00.
				fmt.Fprintf(&b, `\u%04X`, r)
			case r < 0x20 || r == 0x7f:
				fmt.Fprintf(&b, `\x%02X`, r)
			case unicode.IsGraphic(r) || unicode.In(r, unicode.Cf, unicode.Co):
				// MRI's "printable" for UTF-8: graphic, format or private use.
				b.WriteRune(r)
			case r > 0xFFFF:
				fmt.Fprintf(&b, `\u{%X}`, r)
			default:
				fmt.Fprintf(&b, `\u%04X`, r)
			}
		}
	}
	b.WriteByte('"')
	res := b.String()
	for _, c := range []string{string(rune(123)), "$", "@"} {
		res = strings.ReplaceAll(res, "#"+c, `\#`+c)
	}
	return String(res)
}

// rbSleep is Kernel#sleep: MRI's errors, and the whole seconds slept.
func rbSleep(secs float64) Integer {
	if secs < 0 {
		panic(NewArgumentError(Ref(String("time interval must not be negative"))))
	}
	start := time.Now()
	timer := time.NewTimer(time.Duration(secs * float64(time.Second)))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-rbInterruptC():
		rbTakeInterrupt() // Interrupt on the main goroutine; another sleeps on
		<-timer.C
	}
	return Integer(math.Round(time.Since(start).Seconds()))
}

func rbSleepForever() Integer {
	<-rbInterruptC()
	rbTakeInterrupt()
	select {}
}

// Frozen Arrays and Hashes, by pointer (decision 96). rbAnyFrozen keeps a
// program that never freezes one to an atomic load per mutation.
// ponytail: frozen objects stay reachable from the set; use weak pointers if that leak shows up.
var (
	rbAnyFrozen  atomic.Bool
	rbFrozenObjs sync.Map
)

func rbFreeze(p any) {
	rbFrozenObjs.Store(p, true)
	rbAnyFrozen.Store(true)
}

func rbIsFrozen(p any) bool {
	if !rbAnyFrozen.Load() {
		return false
	}
	_, ok := rbFrozenObjs.Load(p)
	return ok
}

// rbFrozenCheck raises MRI's FrozenError before a mutation of a frozen p.
func rbFrozenCheck(p any) {
	if rbIsFrozen(p) {
		panic(NewFrozenError(Ref("can't modify frozen " + String(rbClassName(p)) + ": " + rbInspect(p))))
	}
}

// rbIvarNames is Kernel#instance_variables: the ivars inspect would list (one not assigned yet reads as nil and is left out).
func rbIvarNames(a any) *Array[Symbol] {
	out := &Array[Symbol]{}
	if o, ok := a.(interface{ _Ivars() []rbIvar }); ok {
		for _, iv := range o._Ivars() {
			if iv.opt || iv.val != nil && !iv.isNil {
				*out = append(*out, Symbol(iv.name))
			}
		}
	}
	return out
}

// rbIvarName reads an ivar name argument (Symbol or String), checking MRI's "@" form.
func rbIvarName(name any) string {
	var s string
	switch n := rbUnbox(name).(type) {
	case Symbol, String:
		s = fmt.Sprint(n)
	default:
		panic(NewTypeError(Ref(rbInspect(name) + " is not a symbol nor a string")))
	}
	if !strings.HasPrefix(s, "@") || strings.HasPrefix(s, "@@") || len(s) < 2 {
		panic(NewNameError(Ref(String("'" + s + "' is not allowed as an instance variable name"))))
	}
	return s
}

// rbIvarGet is Kernel#instance_variable_get: nil for an ivar the class never has.
func rbIvarGet(a, name any) any {
	n := rbIvarName(name)
	if o, ok := a.(interface{ _Ivars() []rbIvar }); ok {
		for _, iv := range o._Ivars() {
			if iv.name == n && !iv.isNil {
				return iv.val
			}
		}
	}
	return nil
}

// rbIvarDefined is Kernel#instance_variable_defined?.
func rbIvarDefined(a, name any) bool {
	n := rbIvarName(name)
	return slices.Contains(*rbIvarNames(a), Symbol(n))
}

// rbIvarSet is Kernel#instance_variable_set over the generated _IvarSet; an ivar the class lacks cannot be added.
func rbIvarSet(a, name, v any) any {
	n := rbIvarName(name)
	rbFrozenCheck(a)
	o, ok := a.(interface{ _IvarSet(string, any) bool })
	if !ok || !o._IvarSet(n, rbUnbox(v)) {
		panic(NewNameError(Ref(String("rb2go: " + rbClassName(a) + " has no instance variable " + n + " (the closed world fixes each class's ivars)"))))
	}
	return v
}

// rbIvarAssign converts an untyped value to an ivar's type, as instance_variable_set writes it.
func rbIvarAssign[T any](dst *T, v any, name string) {
	x, ok := rbConv[T](v)
	if !ok {
		panic(NewTypeError(Ref(String("rb2go: " + rbClassName(v) + " cannot be stored in " + name))))
	}
	*dst = x
}

func rbIvarAssignOpt[T any](dst **T, v any, name string) {
	if v == nil {
		*dst = nil
		return
	}
	var x T
	rbIvarAssign(&x, v, name)
	*dst = &x
}
