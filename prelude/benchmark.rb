# rbs_inline: enabled

# Benchmark's wall-clock timer, CPU-time measurement, and bm/bmbm reports; always defined.

# runtime.GC(), for Benchmark.bmbm's rehearsal/real-pass boundary.
module GC
  #: () -> void
  def self.start = %x{ runtime.GC() }
end

module Benchmark
  CAPTION = "      user     system      total        real\n" #: String
  FORMAT = "%10.6u %10.6y %10.6t %10.6r\n" #: String

  #: () { () -> void } -> Float
  def self.realtime
    start = Time.now
    yield
    Time.now - start
  end

  # getrusage(2) (RUSAGE_SELF/RUSAGE_CHILDREN) for CPU time, wall-clock for real; microsecond granularity, coarser than MRI's.

  #: (?String) { () -> void } -> Tms
  def self.measure(label = "") = %x{
    u0, s0, cu0, cs0 := rbBenchRusage()
    r0 := time.Now()
    blk()
    real := time.Since(r0).Seconds()
    u1, s1, cu1, cs1 := rbBenchRusage()
    return NewBenchmark_Tms(Float(u1-u0), Float(s1-s0), Float(cu1-cu0), Float(cs1-cs0), Float(real), label)
  }

  #: (?Integer) { (Report) -> void } -> Array[Tms]
  def self.bm(width = 0)
    report = Report.new(width + 1)
    yield report
    print(" " * report.width + CAPTION)
    report.list.each { |t| print(t.label.ljust(report.width) + t.to_s) }
    report.list
  end

  # Rehearsal pass warms up allocation/GC before the timed pass; GC.start runs between reports in the real pass, as MRI's, though Go's GC doesn't behave like MRI's.

  #: (?Integer) { (Job) -> void } -> Array[Tms]
  def self.bmbm(width = 0)
    job = Job.new(width)
    yield job
    w = job.width + 1

    puts("Rehearsal ".ljust(w + CAPTION.length, "-"))
    sum = Tms.new
    i = 0
    while i < job.size
      label = job.label_at(i)
      res = Benchmark.measure { job.run_at(i) }
      print(label.ljust(w) + res.to_s)
      sum = sum + res
      i += 1
    end
    ets = sum.format("total: %tsec")
    print((" " + ets + "\n\n").rjust(w + CAPTION.length + 2, "-"))

    print(" " * w + CAPTION)
    results = [] #: Array[Tms]
    i = 0
    while i < job.size
      label = job.label_at(i)
      GC.start
      res = Benchmark.measure(label) { job.run_at(i) }
      print(label.ljust(w) + res.to_s)
      results << res
      i += 1
    end
    results
  end

  # A data object holding the times of a benchmark measurement.
  class Tms < Object
    CAPTION = "      user     system      total        real\n" #: String
    FORMAT = "%10.6u %10.6y %10.6t %10.6r\n" #: String

    attr_reader :utime #: Float
    attr_reader :stime #: Float
    attr_reader :cutime #: Float
    attr_reader :cstime #: Float
    attr_reader :real #: Float
    attr_reader :total #: Float
    attr_reader :label #: String

    #: (?Float, ?Float, ?Float, ?Float, ?Float, ?String) -> void
    def initialize(utime = 0.0, stime = 0.0, cutime = 0.0, cstime = 0.0, real = 0.0, label = "")
      @utime = utime
      @stime = stime
      @cutime = cutime
      @cstime = cstime
      @real = real
      @label = label
      @total = utime + stime + cutime + cstime
    end

    #: (Tms) -> Tms
    def +(other) = Tms.new(utime + other.utime, stime + other.stime, cutime + other.cutime, cstime + other.cstime, real + other.real)

    #: (Tms) -> Tms
    def -(other) = Tms.new(utime - other.utime, stime - other.stime, cutime - other.cutime, cstime - other.cstime, real - other.real)

    #: (Float) -> Tms
    def *(x) = Tms.new(utime * x, stime * x, cutime * x, cstime * x, real * x)

    #: (Float) -> Tms
    def /(x) = Tms.new(utime / x, stime / x, cutime / x, cstime / x, real / x)

    #: (Integer) -> Tms
    def __mul_integer(n) = self * n.to_f

    #: (Integer) -> Tms
    def __div_integer(n) = self / n.to_f

    # %u %y %U %Y %t %r %n take sprintf flags/width/precision; anything else (a literal %%) is left for the final String#% pass over *args, as MRI's.

    #: (?String?, *untyped) -> String
    def format(fmt = nil, *args)
      explicit = !fmt.nil?
      str = fmt || FORMAT
      out = "" #: String
      i = 0
      n = str.length
      while i < n
        c = str[i].to_s
        if c == "%"
          spec = "%"
          i += 1
          while i < n && "-+.0123456789".include?(str[i].to_s)
            spec += str[i].to_s
            i += 1
          end
          if i < n
            d = str[i].to_s
            case d
            when "u" then out += (spec + "f") % utime
            when "y" then out += (spec + "f") % stime
            when "U" then out += (spec + "f") % cutime
            when "Y" then out += (spec + "f") % cstime
            when "t" then out += (spec + "f") % total
            when "r" then out += "(" + ((spec + "f") % real) + ")"
            when "n" then out += (spec + "s") % label
            else
              out += spec + d
            end
            i += 1
          else
            out += spec
          end
        else
          out += c
          i += 1
        end
      end
      explicit ? (out % args) : out
    end

    #: () -> String
    def to_s = format

    #: () -> Array[untyped]
    def to_a = [label, utime, stime, cutime, cstime, real]
  end

  # Sequential reports for Benchmark.bm: each #report measures and prints immediately, matching MRI's Report#item.

  class Report < Object
    attr_reader :width #: Integer
    attr_reader :list #: Array[Tms]

    #: (?Integer) -> void
    def initialize(width = 0)
      @width = width
      @list = [] #: Array[Tms]
    end

    #: (?String) { () -> void } -> Tms
    def report(label = "", &blk)
      @width = label.length if label.length > @width
      res = Benchmark.measure(label, &blk)
      @list << res
      res
    end
  end

  # Registers labelled blocks for bmbm without running them: storing a `&block` param is a compile error (README decision 29), so #report stashes the closure in a raw Go slice and #run_at is the only way to call it back.

  # @go_type struct { width Integer; items []rbBenchJobItem }
  class Job < Object
    #: (?Integer) -> Job
    def self.new(width = 0) = %x{ return &Benchmark_Job{width: width} }

    #: () -> Integer
    def width = %x{ self.width }

    #: () -> Integer
    def size = %x{ Integer(len(self.items)) }

    #: (Integer) -> String
    def label_at(i) = %x{ String(self.items[i].label) }

    #: (Integer) -> void
    def run_at(i) = %x{ self.items[i].fn() }

    #: (?String) { () -> void } -> void
    def report(label = "") = %x{
      if Integer(len(string(label))) > self.width {
        self.width = Integer(len(string(label)))
      }
      self.items = append(self.items, rbBenchJobItem{label: string(label), fn: blk})
    }
  end
end
