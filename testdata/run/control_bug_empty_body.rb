# skip: an empty statement body (rescue that swallows, if/unless/elsif/else, while/until, when, ensure) crashes the compiler with a nil pointer dereference; genStmts misses the typed-nil *StatementsNode

# rbs_inline: enabled

#: (Integer) -> Integer
def risky(n)
  raise ArgumentError, "neg" if n < 0
  n
end

begin
  risky(-1)
rescue ArgumentError
end
puts "swallowed"

#: (Integer) -> void
def quiet(n)
  puts risky(n)
rescue ArgumentError => e
end
quiet(1)
quiet(-1)

i = 0
if i > 2
end
unless i.zero?
end
if i > 5
  puts "big"
elsif i > 3
else
end
while i > 5
end
i += 1 until i >= 3
case i
when 1, 2
when 3 then puts "three"
else
end
begin
  puts "body"
ensure
end
x = if i > 5 then end
puts x.inspect
puts "done #{i}"
