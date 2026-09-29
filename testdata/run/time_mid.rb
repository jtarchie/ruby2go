# rbs_inline: enabled

# Time.new's 7th arg is a zone: a "+HH:MM"/"UTC"/"Z"/military-letter String, or `in:` as a trailing Hash (decision 23/39).
t = Time.new(2024, 3, 10, 1, 30, 0, "+09:00")
puts t, t.utc_offset, t.utc?
puts Time.new(2024, 1, 1, 0, 0, 0, "UTC").utc?
puts Time.new(2024, 1, 1, 0, 0, 0, "Z").utc?
puts Time.new(2024, 1, 1, 0, 0, 0, "A").utc_offset
puts Time.new(2024, 1, 1, 0, 0, 0, "-00:00").utc?
puts Time.at(1_700_000_000, in: "+05:30")
puts Time.now(in: "+09:00").class, Time.now(in: "+09:00").utc_offset

begin
  Time.new(2024, 1, 1, 0, 0, 0, "bogus")
rescue ArgumentError => e
  puts "err: #{e.message}"
end
begin
  Time.new(2024, 1, 1, 0, 0, 0, "+25:00")
rescue ArgumentError => e
  puts "err: #{e.message}"
end

u = Time.utc(2024, 6, 1, 12, 0, 0)
puts u.localtime(3600)
puts u.getlocal(3600).utc_offset
puts u.getlocal("+05:00").utc_offset
puts u.getutc.utc?

r = Time.at(1.5)
puts r.round.to_f, r.floor.to_f, r.ceil.to_f, r.round(2).to_f
f = Time.at(1.23456)
puts f.round(2).to_f, f.floor(2).to_f, f.ceil(2).to_f
n = Time.at(-1.5)
puts n.round.to_f

puts Time.at(0).tv_sec
puts Time.at(1, in: "UTC").tv_usec
puts Time.at(1, in: "UTC").tv_nsec
puts Time.new(2024, 1, 1, 0, 0, 0, "UTC").dst?
puts Time.new(2024, 1, 1, 0, 0, 0, "UTC").isdst

ct = Time.new(2024, 1, 1, 0, 0, 0, "+09:00")
puts ct.ctime, ct.asctime

require "date"
d = Time.new(2024, 3, 15, 10, 0, 0, "+02:00").to_date
puts d, d.class

# Process clocks (decision 78): only differences of the monotonic clock mean anything.
clk0 = Process.clock_gettime(Process::CLOCK_MONOTONIC)
clk1 = Process.clock_gettime(Process::CLOCK_MONOTONIC)
p clk1 >= clk0, clk0.class, Process.clock_gettime(Process::CLOCK_REALTIME) > 1_700_000_000, Process.pid > 0
p Process.clock_gettime(Process::CLOCK_PROCESS_CPUTIME_ID) >= 0, [Process::CLOCK_REALTIME, Process::CLOCK_MONOTONIC]
begin
  Process.clock_gettime(99)
rescue SystemCallError => e
  p e.class
end
