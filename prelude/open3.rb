# rbs_inline: enabled

module Process
  # @go_type struct { pid int; code int }
  class Status < Object
    #: () -> Integer
    def pid = %x{ Integer(self.pid) }

    #: () -> Integer
    def exitstatus = %x{ Integer(self.code) }

    #: () -> bool
    def success? = %x{ Boolean(self.code == 0) }

    #: () -> Integer
    def to_i = exitstatus

    #: () -> String
    def to_s = "pid #{pid} exit #{exitstatus}"

    #: () -> String
    def inspect = "#<Process::Status: #{to_s}>"
  end

  # @go_type struct { cmd *exec.Cmd; status *Process_Status }
  class Waiter < Object
    #: () -> Integer
    def pid = %x{ Integer(self.cmd.Process.Pid) }

    #: () -> Process::Status
    def value = %x{ return rbOpen3Wait(self) }
  end
end

# Open3 execs argv directly, no shell string form; popen3 is block-only (no finalizer to reap an unwaited process).
module Open3
  #: (String, *String) -> [String, Process::Status]
  def self.capture2(cmd, *args) = %x{
    s, st := rbOpen3Capture2(string(cmd), rest_)
    return Tuple2[String, *Process_Status]{F0: s, F1: st}
  }

  #: (String, *String) -> [String, Process::Status]
  def self.capture2e(cmd, *args) = %x{
    s, st := rbOpen3Capture2e(string(cmd), rest_)
    return Tuple2[String, *Process_Status]{F0: s, F1: st}
  }

  #: (String, *String) -> [String, String, Process::Status]
  def self.capture3(cmd, *args) = %x{
    o, e, st := rbOpen3Capture3(string(cmd), rest_)
    return Tuple3[String, String, *Process_Status]{F0: o, F1: e, F2: st}
  }

  #: [T] (String, *String) { (Open3::Writer, Open3::Reader, Open3::Reader, Process::Waiter) -> T } -> T
  def self.popen3(cmd, *args) = %x{
    w, o, e, wt := rbOpen3Popen3Start(string(cmd), rest_)
    defer rbOpen3Popen3Close(w, o, e, wt)
    return blk(w, o, e, wt)
  }

  # @go_type struct { w io.WriteCloser; closed bool }
  class Writer < Object
    include IOWritable

    #: (untyped) -> Integer
    def write(x) = %x{
      if self.closed {
        panic(NewIOError(Ref[String]("closed stream")))
      }
      s := string(rbToS(x))
      _, err := self.w.Write([]byte(s))
      if err != nil {
        panic(NewIOError(Ref(String(err.Error()))))
      }
      return Integer(len(s))
    }

    #: () -> nil
    def close = %x{ rbOpen3CloseW(self) }

    #: () -> bool
    def closed? = %x{ Boolean(self.closed) }
  end

  # @go_type struct { r *bufio.Reader; c io.Closer; closed bool }
  class Reader < Object
    include IOReadable

    #: () -> String?
    def gets = %x{
      if self.closed {
        panic(NewIOError(Ref[String]("closed stream")))
      }
      line, err := self.r.ReadString('\\n')
      if line == "" && err != nil {
        return nil
      }
      s := String(line)
      return &s
    }

    #: () -> String
    def read = %x{
      if self.closed {
        panic(NewIOError(Ref[String]("closed stream")))
      }
      b, _ := io.ReadAll(self.r)
      return String(b)
    }

    #: () -> bool
    def eof? = %x{
      if self.closed {
        return true
      }
      _, err := self.r.Peek(1)
      return Boolean(err != nil)
    }

    #: () -> nil
    def close = %x{ rbOpen3CloseR(self) }

    #: () -> bool
    def closed? = %x{ Boolean(self.closed) }
  end
end
