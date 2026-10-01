# rbs_inline: enabled
# Process.spawn, wait, wait2, waitpid and Kernel#exec (decision 107).

pid = Process.spawn("sh", "-c", "exit 3")
puts "wait returns the pid: #{Process.wait(pid) == pid}"
puts "status: #{$?&.exitstatus} success? #{$?&.success?}"

pid = Process.spawn("true")
reaped, status = Process.wait2(pid)
puts "wait2: #{reaped == pid} #{status.exitstatus}"

a = Process.spawn("sh", "-c", "exit 1")
b = Process.spawn("sh", "-c", "exit 2")
codes = [Process.waitpid(a), Process.wait(b)].map { |p| p == a ? 1 : 2 }
puts "two children: #{codes.inspect}"

begin
  Process.wait
rescue SystemCallError => e
  puts "#{e.class}: #{e.message}"
end

begin
  Process.spawn("no-such-program-xyz")
rescue SystemCallError => e
  puts "#{e.class}: #{e.message}"
end

begin
  exec("no-such-program-xyz")
rescue SystemCallError => e
  puts "#{e.class}: #{e.message}"
end

puts "before exec"
$stdout.flush # MRI does not flush its buffer before exec
exec("echo", "after exec")
puts "never printed"
