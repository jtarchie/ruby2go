# skip: a value-returning block (map) inside a begin/rescue/ensure body writes its value to the enclosing function's result variable, so the Go does not parse or build

# rbs_inline: enabled

ds = [2, 5] #: Array[Integer]
begin
  q = ds.map { |d| 10 / d }
  puts q.inspect
rescue ZeroDivisionError => e
  puts e.message
end

#: (Array[Integer]) -> Array[Integer]
def halves(list)
  list.map { |d| d / 2 }
ensure
  puts "done"
end
puts halves([4, 6]).inspect
