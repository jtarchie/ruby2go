# skip: in an iterator method (yields, compiles to iter.Seq), a `return` or a caller's `break` passing through a yield inside begin/ensure only leaves the begin's Go func literal, so the loop keeps going ("got 2", "loop done"; after a break Go panics: range function continued iteration, exit 1)

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
