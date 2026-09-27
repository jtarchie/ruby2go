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
