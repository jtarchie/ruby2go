# rbs_inline: enabled

# Errno::ESRCH and Errno::EPERM, Process.kill's errors: loaded when a program names either (decision 241), so other
# programs do not carry two more exception classes. rbSysErr builds them through rbKillErrnoHook.
module Errno
  class ESRCH < SystemCallError
    Errno = ::Errno::ESRCH.new.errno.to_i #: Integer
  end
  class EPERM < SystemCallError
    Errno = ::Errno::EPERM.new.errno.to_i #: Integer
  end
end

%x{
var _ = func() bool {
	rbKillErrnoHook = func(err error, loc string) any {
		if errors.Is(err, syscall.ESRCH) {
			return NewErrno_ESRCH(Ref(String("No such process" + loc)))
		}
		return NewErrno_EPERM(Ref(String("Operation not permitted" + loc)))
	}
	return true
}()
}
