# rbs_inline: enabled

# twice/shout reject nil: a local assigned on every path must stay non-optional.

#: (Integer) -> Integer
def twice(n)
  n * 2
end

#: (String) -> String
def shout(s)
  s.upcase
end

#: (bool) -> void
def both_branches(flag)
  if flag
    a = 1
  else
    a = 2
  end
  puts twice(a)
end
both_branches(true)
both_branches(false)

#: (Integer) -> void
def case_else(n)
  case n
  when 1 then s = "one"
  else s = "many"
  end
  puts shout(s)
end
case_else(1)
case_else(2)

#: (Integer) -> void
def guarded(n)
  if n > 0
    v = n
  else
    return
  end
  puts twice(v)
end
guarded(4)
guarded(-1)

#: (Integer) -> void
def raised(n)
  if n > 0
    v = n
  else
    raise ArgumentError, "neg"
  end
  puts twice(v)
end
raised(5)

#: () -> void
def countdown
  i = 3
  while true
    last = i
    i -= 1
    break if i == 0
  end
  puts twice(last)
end
countdown

#: (Integer) -> void
def rescued(n)
  begin
    got = n
    raise ArgumentError, "big" if n > 1
    puts twice(got)
  rescue ArgumentError
    puts twice(got)
  end
end
rescued(1)
rescued(2)

#: () -> void
def ensured
  begin
    puts "body"
  ensure
    e = 5
  end
  puts twice(e)
end
ensured

#: (bool) -> void
def narrowed(flag)
  if flag
    a = 3
    puts twice(a)
    a += 1
    puts twice(a)
  end
  puts a.inspect
end
narrowed(true)
narrowed(false)

[1, 2].each do |k|
  if k == 1
    m = "set"
  end
  puts m.inspect
end

i = 0
while i < 2
  if i == 0
    z = 1
  end
  puts z.inspect
  i += 1
end

w = 1 unless [1].empty?
puts w.inspect
