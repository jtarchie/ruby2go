# rbs_inline: enabled

# Kernel#trap / Signal.trap and Process.kill (decision 100).
p trap("USR1") { |s| puts "got #{s}" }
trap("USR1", "IGNORE")
p trap("USR1", "DEFAULT")
p Signal.trap("TERM") { |_| nil }
begin
  trap("FOO") { |_| nil }
rescue ArgumentError => e
  p e.message
end
begin
  trap("KILL") { |_| nil }
rescue SystemCallError => e
  p e.class, e.message
end
begin
  trap(99) { |_| nil }
rescue ArgumentError => e
  p e.message
end
q = Queue.new #: Queue[Integer]
trap(:USR2) { |s| q << s }
p Process.kill("USR2", Process.pid)
p((q.pop || 0) > 0)
trap("INT", "IGNORE")
Process.kill(:INT, Process.pid)
sleep(0.05)
trap("EXIT") { |_| puts "exit trap" }
puts "end"
