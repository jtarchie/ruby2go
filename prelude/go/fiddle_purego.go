//go:build rugaroo_prelude && !rugaroo_cgo

package prelude

import (
	"fmt"
	"math"
	"runtime"
	"slices"
	"structs"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Fiddle's backend over purego, without cgo (decision 326): the default; --cgo swaps in fiddle_cgo.go (decision 426).

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

// rbFiddleCall calls fn with in and fl (float64 bits) in registers (a float result is in f either way); RegisterFunc's reflect costs ~10x the call (decision 329).
// The argument block is heap memory, not the stack, since a callback into Go during the call may move the goroutine's
// stack. slot caches one per C function, taken with an atomic swap (a sync.Pool round trip cost more than half the
// call); a concurrent call on another thread finds it empty and makes its own. errno is read on the calling thread
// right after the call: by purego's trampoline on darwin, by a second call elsewhere.
func rbFiddleCall(slot *atomic.Pointer[rbFiddleArgs], fn uintptr, in *[8]uintptr, fl *[8]uint64, _ bool) (r uintptr, f uint64, errno Integer) {
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

// rbFiddleSpread: purego's backend passes register arguments only, no varargs (decision 326).
const rbFiddleSpread = false

// rbFiddleCallSpread is never reached: rbFiddlePrepare refuses what it would take.
func rbFiddleCallSpread(*atomic.Pointer[rbFiddleArgs], uintptr, []Integer, []uintptr, int, bool, bool) (uintptr, uint64, Integer) {
	panic(NewNotImplementedError(Ref(String("rugaroo: a Fiddle call with stack or variadic arguments needs --cgo (decision 426)"))))
}

// rbFiddleErrnoSym is libc's function returning errno's address.
func rbFiddleErrnoSym() string {
	if runtime.GOOS == "darwin" {
		return "__error"
	}
	return "__errno_location"
}

// rbFiddleErrnoFn is libc's errno accessor, called after each C call off darwin.
var rbFiddleErrnoFn = rbFiddleLibc(rbFiddleErrnoSym())

// rbFiddleCallback is a C function pointer calling blk (Closure); purego caps them at 2000, never frees, returns no floats.
func rbFiddleCallback(args *Array[Integer], ret Integer, blk func(*Array[any]) any) Integer {
	rbFiddleCheckType(ret)
	if rbFiddleIsFloat(ret) {
		panic(NewNotImplementedError(Ref(String("rugaroo: a Fiddle::Closure cannot return float or double (purego)"))))
	}
	types := slices.Clone(args.s)
	rbFiddleCheckArgs(types, false)
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

// rbFiddleLock runs at the defer statement and rbFiddleUnlock at return, even when a callback's Ruby exception unwinds through the call; the pair keeps rbFiddleCall's suppression comments on their lines.
func rbFiddleLock() struct{} {
	runtime.LockOSThread()
	return struct{}{}
}

func rbFiddleUnlock(struct{}) { runtime.UnlockOSThread() }
