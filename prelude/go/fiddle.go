//go:build rugaroo_prelude

package prelude

import (
	"fmt"
	"math"
	"runtime"
	"slices"
	"strconv"
	"structs"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Fiddle over purego without cgo (#32, decision 326); addresses are Integers, Pointer and friends are Ruby (fiddle.rb).

// rbFiddleAt converts through memory: vet's unsafeptr flags a direct uintptr conversion, but this memory is never Go's.
// Address 0 is MRI's DLError, never a Go nil dereference.
func rbFiddleAt(addr Integer) unsafe.Pointer {
	if addr == 0 {
		panic(rbFiddleDLError("NULL pointer dereference"))
	}
	p := uintptr(addr)
	return *(*unsafe.Pointer)(unsafe.Pointer(&p)) //nolint:gosec // C memory, never Go's
}

// rbFiddleDLError is Fiddle::DLError with msg.
func rbFiddleDLError(msg string) any {
	return NewFiddle_DLError(Ref(String(msg)))
}

// rbFiddleDefault is RTLD_DEFAULT as a variable: on darwin the constant (-2 as a uintptr) overflows Integer.
var rbFiddleDefault = uintptr(purego.RTLD_DEFAULT)

// rbFiddleDlopen is dlopen(3); a nil path is the program itself (every loaded library, RTLD_DEFAULT).
func rbFiddleDlopen(path *String, flags Integer) Integer {
	if path == nil {
		if runtime.GOOS == "linux" { // MRI's process links libm; a binary without cgo links only libc
			_, _ = purego.Dlopen("libm.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		}
		return Integer(rbFiddleDefault)
	}
	h, err := purego.Dlopen(string(*path), int(flags))
	if err != nil {
		panic(rbFiddleDLError(err.Error()))
	}
	return Integer(h)
}

// rbFiddleDlsym is dlsym(3): a symbol's address, or DLError as MRI words it.
func rbFiddleDlsym(h Integer, name String) Integer {
	addr, err := purego.Dlsym(uintptr(h), string(name))
	if err != nil || addr == 0 {
		panic(rbFiddleDLError("unknown symbol \"" + string(name) + "\""))
	}
	return Integer(addr)
}

// rbFiddleDlclose is dlclose(3).
func rbFiddleDlclose(h Integer) {
	if h != Integer(rbFiddleDefault) {
		if err := purego.Dlclose(uintptr(h)); err != nil {
			panic(rbFiddleDLError(err.Error()))
		}
	}
}

// rbFiddleLibc is libc's name for sym (malloc, free, the errno accessor), looked up once per program; 0 when missing.
func rbFiddleLibc(sym string) uintptr {
	addr, err := purego.Dlsym(purego.RTLD_DEFAULT, sym)
	if err != nil {
		return 0
	}
	return addr
}

// rbFiddleErrnoSym is libc's function returning errno's address.
func rbFiddleErrnoSym() string {
	if runtime.GOOS == "darwin" {
		return "__error"
	}
	return "__errno_location"
}

// Each resolves on its own: a platform missing one fails at that function's first call, not at start.
var (
	rbFiddleMallocFn  = rbFiddleLibc("malloc")
	rbFiddleFreeFn    = rbFiddleLibc("free")
	rbFiddleReallocFn = rbFiddleLibc("realloc")
	rbFiddleErrnoFn   = rbFiddleLibc(rbFiddleErrnoSym())
)

// rbFiddleLibcFn is fn, or DLError naming sym when libc lacks it (calling address 0 would crash).
func rbFiddleLibcFn(fn uintptr, sym string) uintptr {
	if fn == 0 {
		panic(rbFiddleDLError("unknown symbol \"" + sym + "\""))
	}
	return fn
}

// rbFiddleMalloc is libc malloc, so C code may free what Ruby allocated; NoMemoryError when it fails.
func rbFiddleMalloc(n Integer) Integer {
	rbCheckAlloc(int(n), 1)
	addr := rbFiddleCall1(rbFiddleLibcFn(rbFiddleMallocFn, "malloc"), uintptr(n))
	if addr == 0 && n > 0 {
		panic(NewNoMemoryError(Ref(String("failed to allocate memory"))))
	}
	return Integer(addr)
}

// rbFiddleRealloc is libc realloc.
func rbFiddleRealloc(addr, n Integer) Integer {
	rbCheckAlloc(int(n), 1)
	in := [8]uintptr{uintptr(addr), uintptr(n)}
	var fl [8]uint64
	r, _, _ := rbFiddleCall(&rbFiddleLibcSlot, rbFiddleLibcFn(rbFiddleReallocFn, "realloc"), &in, &fl)
	if r == 0 && n > 0 {
		panic(NewNoMemoryError(Ref(String("failed to allocate memory"))))
	}
	return Integer(r)
}

// rbFiddleFree is libc free.
func rbFiddleFree(addr Integer) {
	rbFiddleCall1(rbFiddleLibcFn(rbFiddleFreeFn, "free"), uintptr(addr))
}

// rbFiddleFreeAddr is the address of libc's free (Fiddle::RUBY_FREE's role: a free function C memory can carry).
func rbFiddleFreeAddr() Integer { return Integer(rbFiddleFreeFn) }

// rbFiddleBytes is n bytes at addr.
func rbFiddleBytes(addr, n Integer) []byte {
	return unsafe.Slice((*byte)(rbFiddleAt(addr)), int(n)) //nolint:gosec // C memory the program owns
}

// rbFiddleRead is n bytes at addr as a binary String.
func rbFiddleRead(addr, n Integer) String {
	if n <= 0 {
		return ""
	}
	return String(rbFiddleBytes(addr, n))
}

// rbFiddleCString is the NUL-terminated string at addr.
func rbFiddleCString(addr Integer) String {
	n := 0
	for *(*byte)(rbFiddleAt(addr + Integer(n))) != 0 {
		n++
	}
	return String(rbFiddleBytes(addr, Integer(n)))
}

// rbFiddleWrite copies s to addr.
func rbFiddleWrite(addr Integer, s String) {
	if len(s) > 0 {
		copy(rbFiddleBytes(addr, Integer(len(s))), s)
	}
}

// rbFiddleChar is C's char at addr (Pointer#[]): signed, except on Linux arm64, where char is unsigned.
func rbFiddleChar(addr Integer) Integer {
	b := *(*byte)(rbFiddleAt(addr))
	if runtime.GOOS == "linux" && runtime.GOARCH == "arm64" {
		return Integer(b)
	}
	return Integer(int8(b)) //nolint:gosec // C's signed char
}

// rbFiddleWord is the pointer-sized value at addr (Pointer#ptr).
func rbFiddleWord(addr Integer) Integer {
	return Integer(*(*uintptr)(rbFiddleAt(addr)))
}

// rbFiddleArgs is purego's syscallArgs (syscall.go, v0.11.2, 64-bit): the block its trampoline loads registers from and stores results in.
type rbFiddleArgs struct {
	_ structs.HostLayout

	fn, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15                uintptr
	a16, a17, a18, a19, a20, a21, a22, a23, a24, a25, a26, a27, a28, a29, a30, a31, a32 uintptr
	f1, f2, f3, f4, f5, f6, f7, f8                                                      uint64
	r8                                                                                  uintptr
}

// rbFiddleCgocall is the runtime's C call, which purego itself links to.
//
//go:linkname rbFiddleCgocall runtime.cgocall
func rbFiddleCgocall(fn uintptr, arg unsafe.Pointer) int32

// rbFiddleTrampoline is purego's assembly that moves an rbFiddleArgs into registers, calls, and stores the results (and errno on darwin).
//
//go:linkname rbFiddleTrampoline github.com/ebitengine/purego.syscallXABI0
var rbFiddleTrampoline uintptr

// rbFiddleCall calls fn with in and fl (float64 bits) in registers; RegisterFunc's reflect costs ~10x the call (decision 329).
// The argument block is heap memory, not the stack, since a callback into Go during the call may move the goroutine's
// stack. slot caches one per C function, taken with an atomic swap (a sync.Pool round trip cost more than half the
// call); a concurrent call on another thread finds it empty and makes its own. errno is read on the calling thread
// right after the call: by purego's trampoline on darwin, by a second call elsewhere.
func rbFiddleCall(slot *atomic.Pointer[rbFiddleArgs], fn uintptr, in *[8]uintptr, fl *[8]uint64) (r uintptr, f uint64, errno Integer) {
	s := slot.Swap(nil)
	if s == nil {
		s = new(rbFiddleArgs)
	}
	s.fn = fn
	s.a1, s.a2, s.a3, s.a4, s.a5, s.a6, s.a7, s.a8 = in[0], in[1], in[2], in[3], in[4], in[5], in[6], in[7]
	s.f1, s.f2, s.f3, s.f4, s.f5, s.f6, s.f7, s.f8 = fl[0], fl[1], fl[2], fl[3], fl[4], fl[5], fl[6], fl[7]
	s.r8 = 0
	if runtime.GOOS == "darwin" {
		rbFiddleCgocall(rbFiddleTrampoline, unsafe.Pointer(s)) //nolint:gosec // purego's own call path
		r, f, errno = s.a1, s.f1, Integer(int32(s.a3))         //nolint:gosec // errno is a C int
	} else {
		defer rbFiddleUnlock(rbFiddleLock())
		rbFiddleCgocall(rbFiddleTrampoline, unsafe.Pointer(s)) //nolint:gosec // purego's own call path
		r, f = s.a1, s.f1
		if rbFiddleErrnoFn != 0 {
			s.fn = rbFiddleErrnoFn
			rbFiddleCgocall(rbFiddleTrampoline, unsafe.Pointer(s)) //nolint:gosec // purego's own call path
			errno = Integer(*(*int32)(rbFiddleAt(Integer(s.a1))))  //nolint:gosec // a1 is errno's address
		}
		// the thread unlocks at return (defer above)
	}
	slot.Store(s)
	return r, f, errno
}

// rbFiddleLibcSlot is the argument block libc's malloc, free and realloc share.
var rbFiddleLibcSlot atomic.Pointer[rbFiddleArgs]

// rbFiddleCall1 is a one-argument libc call (malloc, free).
func rbFiddleCall1(fn, a uintptr) uintptr {
	in := [8]uintptr{a}
	var fl [8]uint64
	r, _, _ := rbFiddleCall(&rbFiddleLibcSlot, fn, &in, &fl)
	return r
}

// rbFiddleCFunc is a bound C function: its address and TYPE_* codes; every register-passed signature goes through rbFiddleCall.
type rbFiddleCFunc struct {
	slot  atomic.Pointer[rbFiddleArgs]
	addr  uintptr
	args  []Integer
	ret   Integer
	float bool
}

// rbFiddleIsFloat reports a TYPE_FLOAT or TYPE_DOUBLE.
func rbFiddleIsFloat(t Integer) bool { return t == 7 || t == 8 }

// rbFiddleCheckType rejects what purego cannot pass.
func rbFiddleCheckType(t Integer) {
	switch {
	case t == 9:
		panic(NewNotImplementedError(Ref(String("rugaroo: Fiddle::TYPE_VARIADIC (purego cannot make a C variadic call)"))))
	case t < -6 || t > 11:
		panic(rbFiddleDLError("unknown type: " + strconv.Itoa(int(t))))
	}
}

// rbFiddleCheckArgs rejects argument types purego cannot pass: past 8 int or 8 float the stack layout differs per platform.
func rbFiddleCheckArgs(args []Integer) {
	ni, nf := 0, 0
	for _, t := range args {
		rbFiddleCheckType(t)
		switch {
		case t == 0:
		case rbFiddleIsFloat(t):
			nf++
		default:
			ni++
		}
	}
	if ni > 8 || nf > 8 {
		panic(NewNotImplementedError(Ref(String("rugaroo: Fiddle calls take at most 8 integer and 8 floating-point arguments"))))
	}
}

// rbFiddlePrepare binds addr (Function.new).
func rbFiddlePrepare(addr Integer, args *Array[Integer], ret Integer) any {
	if addr == 0 {
		panic(rbFiddleDLError("NULL function pointer"))
	}
	fn := &rbFiddleCFunc{addr: uintptr(addr), ret: ret, float: rbFiddleIsFloat(ret)}
	rbFiddleCheckArgs(args.s)
	for _, t := range args.s {
		if t != 0 { // (void)
			fn.args = append(fn.args, t)
		}
	}
	rbFiddleCheckType(ret)
	return fn
}

// rbFiddleWord64 is one Ruby value as the 64-bit register C reads type t from.
func rbFiddleWord64(t Integer, v any) (uintptr, float64) {
	var i int64
	var f float64
	switch x := v.(type) {
	case nil:
	case Integer:
		i, f = int64(x), float64(x)
	case Float:
		i, f = int64(x), float64(x)
	case Boolean:
		if x {
			i = 1
		}
	default:
		panic(NewTypeError(Ref(String("rugaroo: Fiddle cannot pass " + rbClassName(v)))))
	}
	if t == 7 { // a float sits in the low half of its register
		return 0, math.Float64frombits(uint64(math.Float32bits(float32(f))))
	}
	return uintptr(i), f //nolint:gosec // C's own wraparound for a negative value
}

// rbFiddleCString is s NUL-terminated in memory C may read during the call; kept (not freed) when the result may point into it.
func rbFiddleCStr(s String, keep bool) (uintptr, []byte) {
	if keep { // ponytail: a void* result may point into the copy (strchr), so it is C memory that leaks; tie it to the Pointer if needed.
		mem := rbFiddleMalloc(Integer(len(s) + 1))
		b := rbFiddleBytes(mem, Integer(len(s)+1))
		b[copy(b, s)] = 0
		return uintptr(mem), nil
	}
	b := make([]byte, len(s)+1)
	copy(b, s)
	return uintptr(unsafe.Pointer(&b[0])), b //nolint:gosec // Go's heap does not move; the caller keeps b alive across the call
}

// rbFiddleInvoke calls a prepared function with Fiddle's argument conversions (Function#call; objects are addresses already)
// and stores errno as Fiddle.last_error.
func rbFiddleInvoke(prepared any, args *Array[any]) any {
	fn, ok := prepared.(*rbFiddleCFunc)
	if !ok {
		panic(rbFiddleDLError("rugaroo: not a prepared Fiddle function"))
	}
	return rbFiddleInvokeArgs(fn, args.s)
}

// rbFiddleFast is Function#call's result without call's *args Array, or f itself when Function#call must run: f is
// a subclass (which may override call), or an argument is an object other than a plain Pointer (decision 329).
func rbFiddleFast(f Fiddle_FunctionI, a ...any) any {
	x, exact := f.(*Fiddle_Function)
	if !exact {
		return f
	}
	fn, ok := x.fn.(*rbFiddleCFunc)
	if !ok || len(a) != len(fn.args) {
		return f
	}
	for k, v := range a {
		switch v.(type) {
		case nil, Integer, Float, String, Boolean:
			continue
		}
		p, ok := v.(*Fiddle_Pointer) // always in a program that calls C: Fiddle.__ret makes them
		if t := fn.args[k]; !ok || rbFiddleIsFloat(t) || t == 11 {
			return f
		}
		a[k] = p.addr
	}
	return Fiddle_Meta___Ret(Fiddle_class, x.ret, rbFiddleInvokeArgs(fn, a))
}

// rbFiddleInvokeArgs converts args as Fiddle does, calls fn, and sets Fiddle.last_error.
func rbFiddleInvokeArgs(fn *rbFiddleCFunc, args []any) any {
	if len(args) != len(fn.args) {
		panic(NewArgumentError(Ref(String("wrong number of arguments (given " + strconv.Itoa(len(args)) + ", expected " + strconv.Itoa(len(fn.args)) + ")"))))
	}
	var in [8]uintptr
	var fl [8]uint64
	var keep [8][]byte // String copies in Go memory, alive until the call returns
	ni, nf := 0, 0
	for k, a := range args {
		t := fn.args[k]
		switch {
		case rbFiddleIsFloat(t):
			_, f := rbFiddleWord64(t, a)
			fl[nf] = math.Float64bits(f)
			nf++
			continue
		case t == 11: // TYPE_BOOL: Ruby truthiness
			b, isBool := a.(Boolean)
			if a != nil && (!isBool || bool(b)) {
				in[ni] = 1
			}
		default:
			if s, isStr := a.(String); isStr && (t == 1 || t == 10) {
				in[ni], keep[ni] = rbFiddleCStr(s, fn.ret == 1)
			} else {
				in[ni], _ = rbFiddleWord64(t, a)
			}
		}
		ni++
	}
	r, f, errno := rbFiddleCall(&fn.slot, fn.addr, &in, &fl)
	runtime.KeepAlive(&keep)
	rbFiddleSetErrno(errno)
	return rbFiddleValue(fn.ret, r, math.Float64frombits(f))
}

// rbFiddleSetErrno is Fiddle.last_error = errno, a thread-local. The Thread's fiddleErrno (errno+1; 0: unknown, which
// Thread#[]= resets it to) says what the last call stored, so an unchanged errno skips the store (decision 329).
func rbFiddleSetErrno(errno Integer) {
	t := rbCurrentThread()
	if t.fiddleErrno.Load() == int64(errno)+1 {
		return
	}
	t.locals.Store("__FIDDLE_LAST_ERROR__", errno)
	t.fiddleErrno.Store(int64(errno) + 1)
}

// rbFiddleValue reads type t from a register; narrow types are extended here, as the ABI leaves upper bits unspecified.
func rbFiddleValue(t Integer, r uintptr, f float64) any {
	switch t {
	case 0:
		return nil
	case 7:
		return Float(math.Float32frombits(uint32(math.Float64bits(f)))) //nolint:gosec // the float's own low half
	case 8:
		return Float(f)
	case 11:
		return Boolean(uint8(r) != 0) //nolint:gosec // C's _Bool is one byte
	case 2:
		return Integer(int8(r)) //nolint:gosec // as above, by width
	case -2:
		return Integer(uint8(r)) //nolint:gosec // as above
	case 3:
		return Integer(int16(r)) //nolint:gosec // as above
	case -3:
		return Integer(uint16(r)) //nolint:gosec // as above
	case 4:
		return Integer(int32(r)) //nolint:gosec // as above
	case -4:
		return Integer(uint32(r)) //nolint:gosec // as above
	}
	return Integer(r) //nolint:gosec // ponytail: an unsigned long above 2**63 wraps (Integer is int64)
}

// rbFiddleCallback is a C function pointer calling blk (Closure); purego caps them at 2000, never frees, returns no floats.
func rbFiddleCallback(args *Array[Integer], ret Integer, blk func(*Array[any]) any) Integer {
	rbFiddleCheckType(ret)
	if rbFiddleIsFloat(ret) {
		panic(NewNotImplementedError(Ref(String("rugaroo: a Fiddle::Closure cannot return float or double (purego)"))))
	}
	types := slices.Clone(args.s)
	rbFiddleCheckArgs(types)
	ni, nf := 0, 0
	for _, t := range types {
		switch {
		case t == 0:
		case rbFiddleIsFloat(t):
			nf++
		default:
			ni++
		}
	}
	run := func(in *[8]uintptr, fl *[8]uint64) uintptr {
		vals := make([]any, 0, len(types))
		ni, nf := 0, 0
		for _, t := range types {
			switch {
			case t == 0:
			case rbFiddleIsFloat(t):
				vals = append(vals, rbFiddleValue(t, 0, math.Float64frombits(fl[nf])))
				nf++
			default:
				vals = append(vals, rbFiddleValue(t, in[ni], 0))
				ni++
			}
		}
		r, _ := rbFiddleWord64(ret, blk(&Array[any]{s: vals}))
		return r
	}
	cb := func(i0, i1, i2, i3, i4, i5, i6, i7 uintptr, f0, f1, f2, f3, f4, f5, f6, f7 float64) uintptr {
		return run(&[8]uintptr{i0, i1, i2, i3, i4, i5, i6, i7}, &[8]uint64{
			math.Float64bits(f0), math.Float64bits(f1), math.Float64bits(f2), math.Float64bits(f3),
			math.Float64bits(f4), math.Float64bits(f5), math.Float64bits(f6), math.Float64bits(f7),
		})
	}
	var addr uintptr
	func() {
		defer func() {
			if e := recover(); e != nil {
				panic(rbFiddleDLError(fmt.Sprint("rugaroo: ", e)))
			}
		}()
		addr = purego.NewCallback(cb)
	}()
	if ni <= rbFiddleIntRegs() && nf <= 8 && rbFiddleCallbackInstalled() { // every argument in a register: no reflect
		for i := range rbFiddleCallbacks {
			if rbPuregoCallbackAddr(i) == addr {
				rbFiddleCallbacks[i].Store(&run)
				break
			}
		}
	}
	return Integer(addr)
}

// rbPuregoCallbackArgs is purego's callbackArgs (syscall_notstackargs.go, v0.11.2): which callback, the saved argument
// registers (8 floating, then the integer ones), and the result.
type rbPuregoCallbackArgs struct {
	index  uintptr
	args   unsafe.Pointer
	result [4]uintptr
}

// rbPuregoCallbackWrap is the Go function purego's callback trampoline calls; purego's own reads every parameter
// through reflect (~1 µs for the 16-register superset).
//
//go:linkname rbPuregoCallbackWrap github.com/ebitengine/purego.callbackWrap_call
var rbPuregoCallbackWrap func(*rbPuregoCallbackArgs)

// rbPuregoCallbackAddr is the C address purego hands out for callback i.
//
//go:linkname rbPuregoCallbackAddr github.com/ebitengine/purego.callbackasmAddr
func rbPuregoCallbackAddr(i int) uintptr

var (
	rbFiddleCallbacks    [2000]atomic.Pointer[func(*[8]uintptr, *[8]uint64) uintptr] // by purego's index; nil: purego's own path
	rbFiddleCallbackOnce sync.Once
	rbFiddleCallbackOK   bool
)

// rbFiddleIntRegs is how many integer arguments the C ABI passes in registers (System V amd64: 6).
func rbFiddleIntRegs() int {
	if runtime.GOARCH == "amd64" {
		return 6
	}
	return 8
}

// rbFiddleCallbackInstalled puts a dispatcher in front of purego's callback function, once: closures rugaroo registered
// read their registers from purego's frame without reflect (decision 329); any other index goes on to purego.
func rbFiddleCallbackInstalled() bool {
	rbFiddleCallbackOnce.Do(func() {
		if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" { // ponytail: other register files and frame layouts keep purego's path
			return
		}
		orig := rbPuregoCallbackWrap
		rbPuregoCallbackWrap = func(a *rbPuregoCallbackArgs) {
			if a.index < uintptr(len(rbFiddleCallbacks)) {
				if run := rbFiddleCallbacks[a.index].Load(); run != nil {
					frame := (*[16]uintptr)(a.args)
					var in [8]uintptr
					var fl [8]uint64
					for k := range fl {
						fl[k] = uint64(frame[k])
					}
					copy(in[:rbFiddleIntRegs()], frame[8:])
					a.result[0] = (*run)(&in, &fl)
					return
				}
			}
			orig(a)
		}
		rbFiddleCallbackOK = true
	})
	return rbFiddleCallbackOK
}

// rbFiddleGetInt reads an integer member of C type t at addr.
func rbFiddleGetInt(addr, t Integer) Integer {
	p := rbFiddleAt(addr)
	switch t {
	case 2:
		return Integer(*(*int8)(p))
	case -2, 11:
		return Integer(*(*uint8)(p))
	case 3:
		return Integer(*(*int16)(p))
	case -3:
		return Integer(*(*uint16)(p))
	case 4:
		return Integer(*(*int32)(p))
	case -4:
		return Integer(*(*uint32)(p))
	}
	return Integer(*(*int64)(p))
}

// rbFiddleSetInt writes an integer member of C type t at addr, wrapping as C does.
func rbFiddleSetInt(addr, t, v Integer) {
	p := rbFiddleAt(addr)
	switch t {
	case 2, -2, 11:
		*(*uint8)(p) = uint8(v) //nolint:gosec // C's truncation
	case 3, -3:
		*(*uint16)(p) = uint16(v) //nolint:gosec // C's truncation
	case 4, -4:
		*(*uint32)(p) = uint32(v) //nolint:gosec // C's truncation
	default:
		*(*int64)(p) = int64(v)
	}
}

// rbFiddleGetFloat reads a float (t 7) or double member at addr.
func rbFiddleGetFloat(addr, t Integer) Float {
	if t == 7 {
		return Float(*(*float32)(rbFiddleAt(addr)))
	}
	return Float(*(*float64)(rbFiddleAt(addr)))
}

// rbFiddleSetFloat writes a float (t 7) or double member at addr.
func rbFiddleSetFloat(addr, t Integer, v Float) {
	if t == 7 {
		*(*float32)(rbFiddleAt(addr)) = float32(v)
		return
	}
	*(*float64)(rbFiddleAt(addr)) = float64(v)
}

// rbFiddleLock runs at the defer statement and rbFiddleUnlock at return, even when a callback's Ruby exception unwinds through the call; the pair keeps rbFiddleCall's suppression comments on their lines.
func rbFiddleLock() struct{} {
	runtime.LockOSThread()
	return struct{}{}
}

func rbFiddleUnlock(struct{}) { runtime.UnlockOSThread() }
