# rbs_inline: enabled

#: (Integer) -> Integer
def risky(n)
  raise ArgumentError, "neg" if n < 0
  n
end

begin
  v = risky(-1) rescue raise(KeyError, "wrapped")
  puts v
rescue KeyError => e
  puts e.message
end
w = risky(2) rescue raise("no")
puts w
