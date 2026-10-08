//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
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
	case *Errno_EINVAL:
		n = syscall.EINVAL
	case *Errno_ECHILD:
		n = syscall.ECHILD
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
	default:
		return nil
	}
	v := Integer(n)
	return &v
}
