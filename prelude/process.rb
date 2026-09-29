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
