# rbs_inline: enabled
require "benchmark"

t = Benchmark::Tms.new(1.5, 0.25, 0.1, 0.05, 2.0, "x")
puts t.utime, t.stime, t.cutime, t.cstime, t.real, t.total, t.label
puts t.to_s
puts t.format
puts t.format("%9.6u %9.6y %9.6t %9.6r %n\n")
p t.to_a

t2 = Benchmark::Tms.new(0.5, 0.05, 0.0, 0.0, 0.6, "y")
p (t + t2).to_a
p (t - t2).to_a
p (t * 2).to_a
p (t / 2).to_a
p (t * 2.0).to_a
p (t / 2.0).to_a

sum = Benchmark::Tms.new
puts sum.total
puts sum.label == ""

# real/utime/stime are non-deterministic, so Benchmark.measure is checked by invariant, not value.
r = Benchmark.measure("work") { 1 + 1 }
puts r.is_a?(Benchmark::Tms)
puts r.label
puts r.utime >= 0.0 && r.stime >= 0.0 && r.real >= 0.0
puts r.total == r.utime + r.stime + r.cutime + r.cstime

# bm/bmbm always print real timings, which would break the byte-for-byte MRI diff; Report/Job (their machinery) are exercised directly instead.

report = Benchmark::Report.new(3)
r1 = report.report("aa") { 1 + 1 }
r2 = report.report("b") { 2 + 2 }
puts report.list.size
puts report.list[0].label
puts report.list[1].label
puts report.width
puts r1.is_a?(Benchmark::Tms)
puts r2.is_a?(Benchmark::Tms)

report2 = Benchmark::Report.new(0)
report2.report("longlabel") { nil }
puts report2.width

job = Benchmark::Job.new(0)
job.report("a") { nil }
job.report("bb") { nil }
puts job.width

GC.start
puts "gc ok"

puts Benchmark::CAPTION == "      user     system      total        real\n"
puts Benchmark::FORMAT == "%10.6u %10.6y %10.6t %10.6r\n"
