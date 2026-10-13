# rbs_inline: enabled

# Errno::ESRCH and Errno::EPERM, Process.kill's errors: loaded when a program names either (decision 241), so other
# programs do not carry two more exception classes. rbSysErr builds them through rbKillErrnoHook.
module Errno
  class ESRCH < SystemCallError; end
  class EPERM < SystemCallError; end
end

%x{
var _ = func() bool {
	rbKillErrnoHook = func(err error) any {
		if errors.Is(err, syscall.ESRCH) {
			return NewErrno_ESRCH(Ref(String("No such process")))
		}
		return NewErrno_EPERM(Ref(String("Operation not permitted")))
	}
	return true
}()
}
