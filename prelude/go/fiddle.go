//go:build rb2go_prelude

package prelude

import (
	"fmt"
	"math"
	"runtime"
	"slices"
	"strconv"
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
	addr, _, _ := purego.SyscallN(rbFiddleLibcFn(rbFiddleMallocFn, "malloc"), uintptr(n))
	if addr == 0 && n > 0 {
		panic(NewNoMemoryError(Ref(String("failed to allocate memory"))))
	}
	return Integer(addr)
}

// rbFiddleRealloc is libc realloc.
func rbFiddleRealloc(addr, n Integer) Integer {
	rbCheckAlloc(int(n), 1)
	r, _, _ := purego.SyscallN(rbFiddleLibcFn(rbFiddleReallocFn, "realloc"), uintptr(addr), uintptr(n))
	if r == 0 && n > 0 {
		panic(NewNoMemoryError(Ref(String("failed to allocate memory"))))
	}
	return Integer(r)
}

// rbFiddleFree is libc free.
func rbFiddleFree(addr Integer) {
	purego.SyscallN(rbFiddleLibcFn(rbFiddleFreeFn, "free"), uintptr(addr))
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

// rbFiddleCFunc fits every register-passed C signature: int and float registers fill independently; unread ones are ignored.
type rbFiddleCFunc struct {
	ints  func(i0, i1, i2, i3, i4, i5, i6, i7 uintptr, f0, f1, f2, f3, f4, f5, f6, f7 float64) uintptr
	flts  func(i0, i1, i2, i3, i4, i5, i6, i7 uintptr, f0, f1, f2, f3, f4, f5, f6, f7 float64) float64
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
		panic(NewNotImplementedError(Ref(String("rb2go: Fiddle::TYPE_VARIADIC (purego cannot make a C variadic call)"))))
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
		panic(NewNotImplementedError(Ref(String("rb2go: Fiddle calls take at most 8 integer and 8 floating-point arguments"))))
	}
}

// rbFiddlePrepare binds addr (Function.new).
func rbFiddlePrepare(addr Integer, args *Array[Integer], ret Integer) any {
	if addr == 0 {
		panic(rbFiddleDLError("NULL function pointer"))
	}
	fn := &rbFiddleCFunc{ret: ret, float: rbFiddleIsFloat(ret)}
	rbFiddleCheckArgs(args.s)
	for _, t := range args.s {
		if t != 0 { // (void)
			fn.args = append(fn.args, t)
		}
	}
	rbFiddleCheckType(ret)
	if fn.float {
		purego.RegisterFunc(&fn.flts, uintptr(addr))
	} else {
		purego.RegisterFunc(&fn.ints, uintptr(addr))
	}
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
		panic(NewTypeError(Ref(String("rb2go: Fiddle cannot pass " + rbClassName(v)))))
	}
	if t == 7 { // a float sits in the low half of its register
		return 0, math.Float64frombits(uint64(math.Float32bits(float32(f))))
	}
	return uintptr(i), f //nolint:gosec // C's own wraparound for a negative value
}

// rbFiddleInvoke returns [result, errno]: errno is read on the calling thread before another call can reset it.
func rbFiddleInvoke(prepared any, args *Array[any]) *Array[any] {
	fn, ok := prepared.(*rbFiddleCFunc)
	if !ok {
		panic(rbFiddleDLError("rb2go: not a prepared Fiddle function"))
	}
	if len(args.s) != len(fn.args) {
		panic(NewArgumentError(Ref(String("wrong number of arguments (given " + strconv.Itoa(len(args.s)) + ", expected " + strconv.Itoa(len(fn.args)) + ")"))))
	}
	var in [8]uintptr
	var fl [8]float64
	ni, nf := 0, 0
	for k, a := range args.s {
		i, f := rbFiddleWord64(fn.args[k], a)
		if rbFiddleIsFloat(fn.args[k]) {
			fl[nf] = f
			nf++
		} else {
			in[ni] = i
			ni++
		}
	}
	runtime.LockOSThread()
	var r uintptr
	var rf float64
	if fn.float {
		rf = fn.flts(in[0], in[1], in[2], in[3], in[4], in[5], in[6], in[7], fl[0], fl[1], fl[2], fl[3], fl[4], fl[5], fl[6], fl[7])
	} else {
		r = fn.ints(in[0], in[1], in[2], in[3], in[4], in[5], in[6], in[7], fl[0], fl[1], fl[2], fl[3], fl[4], fl[5], fl[6], fl[7])
	}
	errno := Integer(0)
	if rbFiddleErrnoFn != 0 {
		p, _, _ := purego.SyscallN(rbFiddleErrnoFn)
		errno = Integer(*(*int32)(rbFiddleAt(Integer(p)))) //nolint:gosec // p is errno's address
	}
	runtime.UnlockOSThread()
	return &Array[any]{s: []any{rbFiddleValue(fn.ret, r, rf), errno}}
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
		panic(NewNotImplementedError(Ref(String("rb2go: a Fiddle::Closure cannot return float or double (purego)"))))
	}
	types := slices.Clone(args.s)
	rbFiddleCheckArgs(types)
	cb := func(i0, i1, i2, i3, i4, i5, i6, i7 uintptr, f0, f1, f2, f3, f4, f5, f6, f7 float64) uintptr {
		in := [8]uintptr{i0, i1, i2, i3, i4, i5, i6, i7}
		fl := [8]float64{f0, f1, f2, f3, f4, f5, f6, f7}
		vals := make([]any, 0, len(types))
		ni, nf := 0, 0
		for _, t := range types {
			switch {
			case t == 0:
			case rbFiddleIsFloat(t):
				vals = append(vals, rbFiddleValue(t, 0, fl[nf]))
				nf++
			default:
				vals = append(vals, rbFiddleValue(t, in[ni], 0))
				ni++
			}
		}
		r, _ := rbFiddleWord64(ret, blk(&Array[any]{s: vals}))
		return r
	}
	var addr uintptr
	func() {
		defer func() {
			if e := recover(); e != nil {
				panic(rbFiddleDLError(fmt.Sprint("rb2go: ", e)))
			}
		}()
		addr = purego.NewCallback(cb)
	}()
	return Integer(addr)
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
