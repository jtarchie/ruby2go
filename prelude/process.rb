# rbs_inline: enabled

# Process: clocks and pid. Process::Status lives with Open3 (open3.rb).
module Process
  # macOS's values, as MRI reports them there; only these three clocks exist.
  CLOCK_REALTIME = 0 #: Integer
  CLOCK_MONOTONIC = 6 #: Integer
  CLOCK_PROCESS_CPUTIME_ID = 12 #: Integer

  # Seconds as a Float. The monotonic clock counts from process start (MRI's
  # from boot), so only differences mean anything, as in MRI. No unit
  # argument: :float_second only.
  #: (Integer) -> Float
  def self.clock_gettime(id) = %x{ return rbClockGettime(int(id)) }

  #: () -> Integer
  def self.pid = %x{ Integer(os.Getpid()) }

  #: () -> Integer
  def self.ppid = %x{ Integer(os.Getppid()) }

  #: () -> Integer
  def self.uid = %x{ Integer(os.Getuid()) }

  #: () -> Integer
  def self.euid = %x{ Integer(os.Geteuid()) }

  #: () -> Integer
  def self.gid = %x{ Integer(os.Getgid()) }

  #: () -> Integer
  def self.egid = %x{ Integer(os.Getegid()) }

  #: () -> Integer
  def self.getpgrp = %x{ Integer(syscall.Getpgrp()) }

  # Process.spawn / wait / wait2 / waitpid (decision 107): a child started like
  # system's (decision 97's shell rule), reaped by wait, which sets $?.
  #: (String, *String) -> Integer
  def self.spawn(cmd, *args) = %x{ return rbSpawn(string(cmd), rest_) }

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
# code compile to __backtick; $? to __last_status.
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

  #: () -> Process::Status?
  def __last_status = %x{
    if st := rbLastStatus.Load(); st != nil {
      return &st
    }
    return nil
  }
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

  # ponytail: rb2go's signal table lacks platform extras (macOS EMT, INFO); add them per GOOS if a program lists them.
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
