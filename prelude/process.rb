# rbs_inline: enabled

# Process: clocks, pid and Status.
module Process
  # @go_type struct { pid int; code int; sig int; core bool }
  class Status < Object
    #: () -> Integer
    def pid = %x{ Integer(self.pid) }

    # nil for a child a signal ended, as MRI.
    #: () -> Integer?
    def exitstatus = %x{
      if self.sig != 0 {
        return rbOpt[Integer]{}
      }
      c := Integer(self.code)
      return rbSome(c)
    }

    #: () -> bool?
    def success? = %x{
      if self.sig != 0 {
        return nil
      }
      b := Boolean(self.code == 0)
      return &b
    }

    #: () -> bool
    def exited? = %x{ Boolean(self.sig == 0) }

    #: () -> bool
    def signaled? = %x{ Boolean(self.sig != 0) }

    #: () -> Integer?
    def termsig = %x{
      if self.sig == 0 {
        return rbOpt[Integer]{}
      }
      s := Integer(self.sig)
      return rbSome(s)
    }

    #: () -> bool
    def stopped? = false

    #: () -> Integer?
    def stopsig = nil

    #: () -> bool
    def coredump? = %x{ Boolean(self.core) }

    # The raw wait status: the signal number, or the exit code in the second byte.
    #: () -> Integer
    def to_i = %x{
      if self.sig != 0 {
        return Integer(self.sig)
      }
      return Integer(self.code << 8)
    }

    #: () -> String
    def to_s = %x{ return rbStatusString(self) }

    #: () -> String
    def inspect = "#<Process::Status: #{to_s}>"
  end

  # macOS's values, as MRI reports them there; only these three clocks exist.
  CLOCK_REALTIME = 0 #: Integer
  CLOCK_MONOTONIC = 6 #: Integer
  CLOCK_PROCESS_CPUTIME_ID = 12 #: Integer

  # Seconds as a Float. The monotonic clock counts from process start (MRI's
  # from boot), so only differences mean anything, as in MRI. No unit
  # argument: :float_second only.
  #: (Integer) -> Float
  def self.clock_gettime(id) = %x{ return rbClockGettime(int(id)) }

  # With a unit (concurrent-ruby's monotonic_time): a Float for the :float_* units, an Integer for the others.
  #: (Integer, Symbol) -> untyped
  def self.__clock_gettime_2(id, unit) = %x{ return rbClockGettimeUnit(int(id), string(unit)) }

  #: () -> Integer
  def self.pid = %x{ Integer(os.Getpid()) }

  #: () -> Integer
  def self.ppid = %x{ Integer(os.Getppid()) }

  # 0 for either argument is the calling process (decision 241).
  #: (Integer, Integer) -> Integer
  def self.setpgid(pid, pgid) = %x{
    if err := syscall.Setpgid(int(pid), int(pgid)); err != nil {
      panic(rbSysErr(err, "rb_proc_setpgid", ""))
    }
    return 0
  }

  #: () -> Integer
  def self.uid = %x{ Integer(os.Getuid()) }

  #: () -> Integer
  def self.euid = %x{ Integer(os.Geteuid()) }

  #: () -> Integer
  def self.gid = %x{ Integer(os.Getgid()) }

  #: () -> Integer
  def self.egid = %x{ Integer(os.Getegid()) }

  #: () -> Array[Integer]
  def self.groups = %x{
    gs, _ := os.Getgroups()
    out := make([]Integer, len(gs))
    for i, g := range gs {
      out[i] = Integer(g)
    }
    return &Array[Integer]{s: out}
  }

  #: () -> Integer
  def self.getpgrp = %x{ Integer(syscall.Getpgrp()) }

  # resource: an Integer or a name (:NOFILE, "CORE").
  #: (untyped) -> [Integer, Integer]
  def self.getrlimit(resource) = %x{
    var lim syscall.Rlimit
    if err := syscall.Getrlimit(rbRlimit(resource), &lim); err != nil {
      panic(NewErrno_EINVAL(Ref(String("Invalid argument - getrlimit"))))
    }
    return Tuple2[Integer, Integer]{Integer(min(lim.Cur, math.MaxInt64)), Integer(min(lim.Max, math.MaxInt64))}
  }

  # Process.spawn / wait / wait2 / waitpid (decision 107): a child started like
  # system's (decision 97's shell rule), reaped by wait, which sets $?.
  #: (String, *String) -> Integer
  def self.spawn(cmd, *args) = %x{ return rbSpawn(string(cmd), rest_) }

  # spawn's general form: an env Hash first, [program, argv0], options (decision 233). Calls whose first argument
  # is not a String reach it (decision 12's `__<name>_<class>`): Homebrew's SystemCommand#exec3.
  #: (Hash[untyped, untyped], *untyped, **untyped) -> Integer
  def self.__spawn_hash(env, *args, **opts) = %x{ return rbSpawnFull(append([]any{env}, rest_...), opts) }

  #: (Array[untyped], *untyped, **untyped) -> Integer
  def self.__spawn_array(cmd, *args, **opts) = %x{ return rbSpawnFull(append([]any{cmd}, rest_...), opts) }

  # A thread that reaps pid; its value is the child's Process::Status, its pid the child's.
  #: (Integer) -> Thread
  def self.detach(pid)
    t = Thread.new { __reap(pid) }
    t.__set_pid(pid)
    t
  end

  #: (Integer) -> Process::Status
  def self.__reap(pid)
    wait(pid)
    __status
  end

  # pid -1 (the default) reaps whichever child finishes first; Errno::ECHILD with none left.
  #: (?Integer) -> Integer
  def self.wait(pid = -1) = %x{ return rbWait(int(pid)) }

  #: (?Integer) -> Integer
  def self.waitpid(pid = -1) = wait(pid)

  #: (?Integer) -> [Integer, Process::Status]
  def self.wait2(pid = -1)
    reaped = wait(pid)
    [reaped, __status]
  end

  #: () -> Process::Status
  def self.__status = %x{ return rbLastStatus.Load() }

  # Signals one process; MRI's return, the count signalled, is always 1 here.
  #: (untyped, Integer) -> Integer
  def self.kill(sig, pid) = %x{
    if err := syscall.Kill(int(pid), rbSignalArg(sig)); err != nil {
      panic(rbSysErr(err, "rb_f_kill", strconv.Itoa(int(pid))))
    }
    return 1
  }
end

# Kernel#system, backticks and $? (decision 97). Backticks and %x() in user
# code compile to __backtick; $? reads rbLastStatusOpt.
module Kernel
  private

  # true on exit 0, false on another status, nil when the command could not run.
  #: (String, *String) -> bool?
  def system(cmd, *args) = %x{ return rbSystem(string(cmd), rest_) }

  #: (String) -> String
  def __backtick(cmd) = %x{ return rbBacktick(string(cmd)) }

  # Replaces the program with the command (decision 107); only a failure to start returns, as Errno::ENOENT.
  #: (String, *String) -> void
  def exec(cmd, *args) = %x{ rbExec(string(cmd), rest_) }
end

# Kernel#trap / Signal.trap (decision 100): a block runs on the signal
# goroutine when the signal arrives; "IGNORE", "DEFAULT" and "EXIT" work.
module Kernel
  private

  #: (untyped) { (Integer) -> void } -> String?
  def trap(sig) = %x{ return rbSetTrap(sig, blk, "") }

  #: (untyped, String) -> String?
  def __trap_2(sig, command) = %x{ return rbSetTrap(sig, nil, string(command)) }
end

module Signal
  #: (untyped) { (Integer) -> void } -> String?
  def self.trap(sig) = %x{ return rbSetTrap(sig, blk, "") }

  #: (untyped, String) -> String?
  def self.__trap_2(sig, command) = %x{ return rbSetTrap(sig, nil, string(command)) }

  # ponytail: rugaroo's signal table lacks platform extras (macOS EMT, INFO); add them per GOOS if a program lists them.
  #: () -> Hash[String, Integer]
  def self.list = %x{
    names := slices.Collect(maps.Keys(rbSignalNums))
    slices.SortFunc(names, func(a, b string) int {
      return cmp.Or(cmp.Compare(rbSignalNums[a], rbSignalNums[b]), cmp.Compare(a, b))
    })
    out := NewHash[String, Integer]()
    for _, n := range names {
      Hash_Op_idxSet(out, String(n), Integer(rbSignalNums[n]))
    }
    return out
  }

  #: (Integer) -> String?
  def self.signame(n) = %x{
    if n == 0 {
      return Ref(String("EXIT"))
    }
    if name := rbSignalName(syscall.Signal(n)); name != strconv.Itoa(int(n)) {
      return Ref(String(name))
    }
    return nil
  }
end
