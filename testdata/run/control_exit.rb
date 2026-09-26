# rbs_inline: enabled

#: (Integer) -> void
def finish(code)
  puts "finishing with #{code}"
  exit code
end

# SystemExit is not a StandardError, so `rescue => e` must not stop the exit
begin
  [1, 2].each do |i|
    puts "loop #{i}"
    finish(3) if i == 2
  end
rescue => e
  puts "never #{e.class}"
end
puts "unreachable"
