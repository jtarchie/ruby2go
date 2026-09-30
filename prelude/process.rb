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

  #: () -> Process::Status?
  def __last_status = %x{
    if st := rbLastStatus.Load(); st != nil {
      return &st
    }
    return nil
  }
end
