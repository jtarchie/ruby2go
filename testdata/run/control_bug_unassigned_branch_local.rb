# rbs_inline: enabled

#: (bool) -> void
def branch_local(flag)
  if flag
    a = 1
  end
  puts a.inspect
end
branch_local(true)
branch_local(false)

#: (Integer) -> void
def loop_local(n)
  i = 0
  while i < n
    last = "v#{i}"
    i += 1
  end
  puts last.inspect
end
loop_local(2)
loop_local(0)

b = 5 if [].size > 0
puts b.inspect

#: (Integer) -> void
def half(n)
  begin
    raise ArgumentError, "odd" if n.odd?
    h = n / 2
  rescue ArgumentError
    puts "rescue sees #{h.inspect}"
  end
  puts h.inspect
end
half(4)
half(3)
