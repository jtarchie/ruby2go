# rbs_inline: enabled

#: (Integer) { (Integer) -> void } -> void
def upto_stop(n)
  i = 0
  while i < n
    begin
      return if i == 2
    ensure
      puts "ensure #{i}"
    end
    yield i
    i += 1
  end
  puts "loop done"
end
upto_stop(4) { |x| puts "got #{x}" }

#: () { (Integer) -> void } -> void
def each_logged
  i = 0
  while i < 3
    i += 1
    begin
      yield i
    ensure
      puts "after #{i}"
    end
  end
  puts "each_logged done"
end
each_logged do |v|
  break if v == 2
  puts "b#{v}"
end
puts "end"
