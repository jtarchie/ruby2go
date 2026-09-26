# skip: rescue or ensure inside a value-returning block (`map do ... rescue/ensure ... end`, or a begin/rescue as the block body) assigns the block value to an empty result name, so the Go does not parse

# rbs_inline: enabled

ds = [2, 0, 5] #: Array[Integer]
q = ds.map do |d|
  10 / d
rescue ZeroDivisionError
  -1
end
puts q.inspect
r = ds.map { |d| begin; 10 / d; rescue ZeroDivisionError; -2; end }
puts r.inspect
s = [2, 5].map do |d|
  10 / d
ensure
  puts "ensure #{d}"
end
puts s.inspect
