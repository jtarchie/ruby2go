# rbs_inline: enabled

class Boom < StandardError; end

#: (Integer) -> Integer
def deep(n)
  puts "enter #{n}"
  raise Boom, "boom at #{n}" if n.zero?
  deep(n - 1)
ensure
  puts "unwind #{n}"
end

begin
  deep(1)
rescue ArgumentError
  puts "wrong class, not rescued"
ensure
  puts "outer ensure"
end
puts "unreachable"
