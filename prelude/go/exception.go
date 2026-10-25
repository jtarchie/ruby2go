//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"strconv"
	"strings"
	"syscall"
)

// rbWithCause gives e the exception being handled (r_ in a rescue clause) as its cause, as MRI's raise does, unless e already has one or is that exception itself.
func rbWithCause(e, handled any) any {
	h, ok := handled.(ExceptionI)
	if !ok || e == handled {
		return e
	}
	if x, ok := e.(interface{ __SetCause(ExceptionI) }); ok {
		x.__SetCause(h)
	}
	return e
}

// rbErrnoOf is SystemCallError#errno: the platform's number for an Errno class rb2go raises.
func rbErrnoOf(e any) *Integer {
	var n syscall.Errno
	switch e.(type) {
	case *Errno_ENOENT:
		n = syscall.ENOENT
	case *Errno_EBADF:
		n = syscall.EBADF
	case *Errno_EEXIST:
		n = syscall.EEXIST
	case *Errno_EACCES:
		n = syscall.EACCES
	case *Errno_EISDIR:
		n = syscall.EISDIR
	case *Errno_ENOTDIR:
		n = syscall.ENOTDIR
	case *Errno_ENOTEMPTY:
		n = syscall.ENOTEMPTY
	case *Errno_EBUSY:
		n = syscall.EBUSY
	case *Errno_ESPIPE:
		n = syscall.ESPIPE
	case *Errno_EROFS:
		n = syscall.EROFS
	case *Errno_ELOOP:
		n = syscall.ELOOP
	case *Errno_EINVAL:
		n = syscall.EINVAL
	case *Errno_ECHILD:
		n = syscall.ECHILD
	case *Errno_ESRCH:
		n = syscall.ESRCH
	case *Errno_EPERM:
		n = syscall.EPERM
	case interface{ _Errno_EAGAIN() *Errno_EAGAIN }: // IO::EAGAINWaitReadable too
		n = syscall.EAGAIN
	case *Errno_ECONNREFUSED:
		n = syscall.ECONNREFUSED
	case *Errno_EADDRINUSE:
		n = syscall.EADDRINUSE
	case *Errno_EADDRNOTAVAIL:
		n = syscall.EADDRNOTAVAIL
	case *Errno_EPIPE:
		n = syscall.EPIPE
	case *Errno_ECONNRESET:
		n = syscall.ECONNRESET
	case *Errno_ECONNABORTED:
		n = syscall.ECONNABORTED
	case *Errno_ENOTCONN:
		n = syscall.ENOTCONN
	case *Errno_EISCONN:
		n = syscall.EISCONN
	case *Errno_EDESTADDRREQ:
		n = syscall.EDESTADDRREQ
	case *Errno_ETIMEDOUT:
		n = syscall.ETIMEDOUT
	case *Errno_EHOSTUNREACH:
		n = syscall.EHOSTUNREACH
	case *Errno_ENETUNREACH:
		n = syscall.ENETUNREACH
	case *Errno_EAFNOSUPPORT:
		n = syscall.EAFNOSUPPORT
	case *Errno_EMFILE:
		n = syscall.EMFILE
	case *Errno_ENOTTY:
		n = syscall.ENOTTY
	case *Errno_ENODEV:
		n = syscall.ENODEV
	case *Errno_ENOSYS:
		n = syscall.ENOSYS
	case *Errno_EOPNOTSUPP:
		n = syscall.EOPNOTSUPP
	case *Errno_ERANGE:
		n = syscall.ERANGE
	default:
		return nil
	}
	v := Integer(n)
	return &v
}

// rbRaiseValue is what a gem's `raise x` or `raise klass, msg` raises when x or klass is held untyped (decision 233):
// an exception as it is, a String as a RuntimeError, a class through its new (with msg when given).
func rbRaiseValue(x any, msg any) ExceptionI {
	switch v := rbUnbox(x).(type) {
	case ExceptionI:
		return v
	case String:
		return NewRuntimeError(Ref(v))
	}
	var args []any
	if msg != nil {
		args = append(args, msg)
	}
	if e, ok := rbDynNew(rbCall, x, args...).(ExceptionI); ok {
		return e
	}
	return NewTypeError(Ref(String("exception class/object expected")))
}

// rbStrerror is C's strerror text for errno n, capitalized as MRI's messages print it.
func rbStrerror(n int) string {
	s := syscall.Errno(n).Error()
	if s == "" {
		return "Unknown error " + strconv.Itoa(n)
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// rbSysMessage is the message of `Errno::X.new(m)` (genNew): MRI's, the error's text then the given one
// (`Invalid argument - x`, `Invalid argument @ location - x`); one rb2go's runtime built already starts with the text.
func rbSysMessage(e any, m *String) *String {
	n := rbErrnoOf(e) // ponytail: SystemCallError itself keeps its message ("unknown error - " in MRI); naming its type here made every program keep the class
	if n == nil {
		return m
	}
	s := rbStrerror(int(*n))
	switch {
	case m == nil:
		return Ref(String(s))
	case strings.HasPrefix(string(*m), "@ "):
		return Ref(String(s + " " + string(*m)))
	case strings.HasPrefix(string(*m), s):
		return m
	}
	return Ref(String(s + " - " + string(*m)))
}

// rbSysLocMessage is `Errno::X.new(m, location)`'s message before rbSysMessage puts the error's text first.
func rbSysLocMessage(loc String, m *String) *String {
	return Ref(String("@ " + string(loc) + " - " + string(rbDerefOrNil(m))))
}
