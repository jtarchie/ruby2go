# skip: Ruby locals named like rb2go's generated Go identifiers collide: `r` read in a rescue body is the recovered panic (prints "boom", not 10), `t1` is redeclared by a lifted temp, `p` in a rescue-modifier fallback is the recovered panic, `ret_` is a rescuing method's named result (go build fails)

# rbs_inline: enabled

r = 10
puts r
begin
  raise "boom"
rescue
  puts r
end

#: (Integer?) -> Integer
def pick(x)
  t1 = 100
  y = x || t1
  y + t1
end
puts pick(nil), pick(1)

#: (Integer) -> Integer
def risky(n)
  raise ArgumentError, "neg" if n < 0
  n
end
p = 7
v = risky(-1) rescue p
puts v

#: (Integer) -> String
def doubled(n)
  ret_ = n * 2
  raise "x" if n < 0
  "v#{ret_}"
rescue
  "rescued"
end
puts doubled(1), doubled(-1)
