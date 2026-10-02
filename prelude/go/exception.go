//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

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
	default:
		return nil
	}
	v := Integer(n)
	return &v
}
