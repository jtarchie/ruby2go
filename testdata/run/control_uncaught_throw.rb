# rbs_inline: enabled

# A throw nothing catches ends the program like an uncaught exception, after ensure.
begin
  puts "before"
  throw :bye, 1
ensure
  puts "ensure"
end
puts "unreachable"
