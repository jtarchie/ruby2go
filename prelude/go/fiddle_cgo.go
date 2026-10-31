//go:build rugaroo_prelude && rugaroo_cgo

package prelude

// #cgo linux LDFLAGS: -ldl
// #include "fiddle_cgo.h"
import "C"

import (
	"math"
	"runtime"
	"slices"
	"sync/atomic"
	"unsafe"
)

// Fiddle's backend over cgo (decision 426): `rugaroo build --cgo` swaps it in for fiddle_purego.go, so purego is not
// linked. C does the calls (errno read in the same frame), dlopen and the callback trampolines.

// rbFiddleDefault is RTLD_DEFAULT as a variable: on darwin the constant (-2 as a uintptr) overflows Integer.
var rbFiddleDefault = uintptr(C.rb_fiddle_default())

// rbFiddleDLErrorText is dlerror(3)'s message, or fallback when it has none.
func rbFiddleDLErrorText(fallback string) string {
	if e := C.dlerror(); e != nil {
		return C.GoString(e)
	}
	return fallback
}

// rbFiddleDlopen is dlopen(3); a nil path is the program itself (every loaded library, RTLD_DEFAULT).
func rbFiddleDlopen(path *String, flags Integer) Integer {
	if path == nil {
		if runtime.GOOS == "linux" { // MRI's process links libm; a Go binary may not
			name := C.CString("libm.so.6")
			C.dlopen(name, C.RTLD_NOW|C.RTLD_GLOBAL)
			C.free(unsafe.Pointer(name))
		}
		return Integer(rbFiddleDefault)
	}
	name := C.CString(string(*path))
	defer C.free(unsafe.Pointer(name))
	h := C.dlopen(name, C.int(flags))
	if h == nil {
		panic(rbFiddleDLError(rbFiddleDLErrorText("dlopen failed: " + string(*path))))
	}
	return Integer(uintptr(h))
}

// rbFiddleDlsym is dlsym(3): a symbol's address, or DLError as MRI words it.
func rbFiddleDlsym(h Integer, name String) Integer {
	addr := rbFiddleLibcAt(uintptr(h), string(name))
	if addr == 0 {
		panic(rbFiddleDLError("unknown symbol \"" + string(name) + "\""))
	}
	return Integer(addr)
}

// rbFiddleLibcAt is dlsym(h, sym), 0 when missing.
func rbFiddleLibcAt(h uintptr, sym string) uintptr {
	name := C.CString(sym)
	defer C.free(unsafe.Pointer(name))
	return uintptr(C.rb_fiddle_dlsym(C.uintptr_t(h), name))
}

// rbFiddleDlclose is dlclose(3).
func rbFiddleDlclose(h Integer) {
	if h != Integer(rbFiddleDefault) && C.rb_fiddle_dlclose(C.uintptr_t(h)) != 0 {
		panic(rbFiddleDLError(rbFiddleDLErrorText("dlclose failed")))
	}
}

// rbFiddleLibc is libc's name for sym (malloc, free, realloc), looked up once per program; 0 when missing.
func rbFiddleLibc(sym string) uintptr { return rbFiddleLibcAt(rbFiddleDefault, sym) }

// rbFiddleArgs is the argument block rb_fiddle_call reads and writes; Go heap memory with no Go pointers in it.
type rbFiddleArgs = C.rb_fiddle_args

// rbFiddleCall calls fn with in and fl (float64 bits) in registers, as fiddle_purego.go's does, but through cgo:
// errno comes back from the same C frame, so no thread lock and no second call. The block is cached per C function
// (slot): an address handed to C escapes, so a stack block would be a heap allocation per call.
func rbFiddleCall(slot *atomic.Pointer[rbFiddleArgs], fn uintptr, in *[8]uintptr, fl *[8]uint64, fret bool) (r uintptr, f uint64, errno Integer) {
	s := slot.Swap(nil)
	if s == nil {
		s = new(rbFiddleArgs)
	}
	for k := range in {
		s.a[k] = C.uintptr_t(in[k])
		s.f[k] = C.uint64_t(fl[k])
	}
	ret := C.int(0)
	if fret {
		ret = 1
	}
	C.rb_fiddle_call(C.uintptr_t(fn), ret, s)
	r, f, errno = uintptr(s.r), uint64(s.rf), Integer(s.err)
	slot.Store(s)
	return r, f, errno
}

// rbFiddleSpread: this backend lays out stack and variadic arguments (rbFiddlePlace).
const rbFiddleSpread = true

// rbFiddleCallSpread: rb_fiddle_call_spread passes the registers and 16 stack words, so stack arguments get 128 bytes.
func rbFiddleCallSpread(slot *atomic.Pointer[rbFiddleArgs], fn uintptr, types []Integer, words []uintptr, nfixed int, variadic, fret bool) (r uintptr, f uint64, errno Integer) {
	s := slot.Swap(nil)
	if s == nil {
		s = new(rbFiddleArgs)
	}
	*s = rbFiddleArgs{}
	rbFiddlePlace(s, types, words, nfixed, variadic)
	ret := C.int(0)
	if fret {
		ret = 1
	}
	C.rb_fiddle_call_spread(C.uintptr_t(fn), ret, s)
	r, f, errno = uintptr(s.r), uint64(s.rf), Integer(s.err)
	slot.Store(s)
	return r, f, errno
}

// rbFiddlePlace is libffi's placement for System V x86-64, AAPCS64 and Apple arm64 (which packs named stack args and stacks every variadic one).
func rbFiddlePlace(s *rbFiddleArgs, types []Integer, words []uintptr, nfixed int, variadic bool) {
	apple := runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
	intRegs := 8
	if runtime.GOARCH == "amd64" {
		intRegs = 6
	}
	stack := (*[len(s.s) * 8]byte)(unsafe.Pointer(&s.s))
	ni, nf, off := 0, 0, 0
	for k, t := range types {
		w := uint64(words[k])
		float := rbFiddleIsFloat(t)
		size := 8
		switch {
		case variadic && k >= nfixed && apple:
			off = (off + 7) &^ 7
		case float && nf < 8:
			s.f[nf] = C.uint64_t(w)
			nf++
			continue
		case !float && ni < intRegs:
			s.a[ni] = C.uintptr_t(w)
			ni++
			continue
		case apple:
			size = rbFiddleSize(t)
			off = (off + size - 1) &^ (size - 1)
		}
		if off+size > len(stack) {
			panic(NewNotImplementedError(Ref(String("rugaroo: a Fiddle call passes at most 128 bytes of stack arguments"))))
		}
		for b := range size { // little-endian: a float's bits and a narrow integer are the low bytes
			stack[off+b] = byte(w >> (8 * b))
		}
		off += size
	}
}

// rbFiddleSize is TYPE_* t's byte size (LP64).
func rbFiddleSize(t Integer) int {
	switch t {
	case 2, -2, 11:
		return 1
	case 3, -3:
		return 2
	case 4, -4, 7:
		return 4
	}
	return 8
}

// rbFiddleCallbackRun is one Closure: the block over the saved argument registers, an integer or a double result.
type rbFiddleCallbackRun = func(in *[8]uintptr, fl *[8]uint64) (uintptr, float64)

var (
	rbFiddleCallbacks    [2000]atomic.Pointer[rbFiddleCallbackRun] // by trampoline index, each table its own
	rbFiddleCallbacksF   [2000]atomic.Pointer[rbFiddleCallbackRun]
	rbFiddleCallbackNext atomic.Int32
	rbFiddleCallbackNxtF atomic.Int32
)

//export rbFiddleCgoCallback
func rbFiddleCgoCallback(n C.int, in *C.uintptr_t, fl *C.uint64_t) C.uintptr_t {
	r, _ := (*rbFiddleCallbacks[n].Load())((*[8]uintptr)(unsafe.Pointer(in)), (*[8]uint64)(unsafe.Pointer(fl)))
	return C.uintptr_t(r)
}

//export rbFiddleCgoCallbackF
func rbFiddleCgoCallbackF(n C.int, in *C.uintptr_t, fl *C.uint64_t) C.double {
	_, f := (*rbFiddleCallbacksF[n].Load())((*[8]uintptr)(unsafe.Pointer(in)), (*[8]uint64)(unsafe.Pointer(fl)))
	return C.double(f)
}

// rbFiddleCallback is a C function pointer calling blk (Closure): one of 2000 C trampolines per result kind (integer,
// double), never freed, as purego's (decision 326) but without reflect and with float and double results.
func rbFiddleCallback(args *Array[Integer], ret Integer, blk func(*Array[any]) any) Integer {
	rbFiddleCheckType(ret)
	types := slices.Clone(args.s)
	rbFiddleCheckArgs(types, false)
	fret := rbFiddleIsFloat(ret)
	run := rbFiddleCallbackRun(func(in *[8]uintptr, fl *[8]uint64) (uintptr, float64) {
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
		r, f := rbFiddleWord64(ret, blk(&Array[any]{s: vals}))
		return r, f
	})
	_, _ = rbFiddleCgoCallback, rbFiddleCgoCallbackF // only C calls them: this keeps them through pruning
	table, next := &rbFiddleCallbacks, &rbFiddleCallbackNext
	if fret {
		table, next = &rbFiddleCallbacksF, &rbFiddleCallbackNxtF
	}
	n := next.Add(1) - 1
	if int(n) >= len(table) {
		panic(rbFiddleDLError("rugaroo: more than 2000 Fiddle closures of one result kind"))
	}
	table[n].Store(&run)
	return Integer(C.rb_fiddle_cb_addr(C.int(n), C.int(rbFiddleBool(fret))))
}

// rbFiddleBool is b as C's 0 or 1.
func rbFiddleBool(b bool) int {
	if b {
		return 1
	}
	return 0
}

// rbFFIStub calls fn through by-value C stub n (ffi_stub.go), each value as the 64-bit word Fiddle would pass, a struct
// result copied to retAddr; anything else is ret's value.
func rbFFIStub(n, fn Integer, types *Array[Integer], vals *Array[any], retAddr, ret Integer) any {
	v := make([]C.uint64_t, max(len(vals.s), 1))
	keep := make([][]byte, len(vals.s)) // String copies in Go memory, alive until the call returns
	for k, a := range vals.s {
		t := types.s[k]
		var w uintptr
		switch {
		case rbFiddleIsFloat(t):
			_, f := rbFiddleWord64(t, a)
			w = uintptr(math.Float64bits(f))
		case t == 11:
			b, isBool := a.(Boolean)
			if a != nil && (!isBool || bool(b)) {
				w = 1
			}
		default:
			if s, isStr := a.(String); isStr && (t == 1 || t == 10) {
				w, keep[k] = rbFiddleCStr(s, ret == 1)
			} else {
				w, _ = rbFiddleWord64(t, a)
			}
		}
		v[k] = C.uint64_t(w)
	}
	var r C.uint64_t
	var errno C.int
	out := unsafe.Pointer(&r)
	if retAddr != 0 {
		out = rbFiddleAt(retAddr)
	}
	C.rb_ffi_stub(C.int(n), C.uintptr_t(fn), &v[0], out, &errno)
	runtime.KeepAlive(keep)
	rbFiddleSetErrno(Integer(errno))
	if retAddr != 0 {
		return nil
	}
	return rbFiddleValue(ret, uintptr(r), math.Float64frombits(uint64(r)))
}
