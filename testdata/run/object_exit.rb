# rbs_inline: enabled

# Kernel#exit from inside an object's method flushes buffered output and sets the status; print and puts on objects.

class Job
  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end

  #: () -> String
  def to_s = "job(#{name})"

  #: (Integer) -> void
  def finish(code)
    print "finishing ", self, " ", code, "\n"
    exit(code)
  end
end

jobs = [Job.new("a"), Job.new("")] #: Array[Job]
puts jobs
puts [jobs, [Job.new("ü")]]
print jobs.first(1).fetch(0), "\n"
puts nil
puts "#{jobs.fetch(1)}|"
jobs.fetch(0).finish(3)
puts "unreachable"
