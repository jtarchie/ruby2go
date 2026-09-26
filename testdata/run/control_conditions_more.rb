# rbs_inline: enabled

# an assignment as the condition: the value decides, the local stays set
h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
if (m = h["a"])
  puts "found #{m + 1}"
end
if (m2 = h["zz"])
  puts "never #{m2}"
else
  puts "missing #{m2.inspect}"
end

# nested ternaries without parentheses associate to the right
x = 4
puts(x > 5 ? "a" : x > 3 ? "b" : "c")
puts(x > 5 ? "a" : x > 4 ? "b" : "c")
puts(x.even? && x > 2 ? "even big" : "other")

# `when` with constant values and expressions on constants is ==, not a type test
LIMIT = 3

#: (Integer) -> String
def vs_const(n)
  case n
  when LIMIT then "at limit"
  when LIMIT * 2 then "double"
  else "no"
  end
end
puts vs_const(3), vs_const(6), vs_const(1)

# several classes in one arm

#: (untyped) -> String
def multi(v)
  case v
  when String, Symbol then "text #{v}"
  when Integer, Float then "num #{v}"
  when nil then "nil"
  else "?"
  end
end
puts multi("s"), multi(:y), multi(2), multi(2.5), multi(nil), multi([1])

#: (Integer?) -> String
def nil_or_zero(v)
  case v
  when nil, 0 then "empty"
  else "val #{v}"
  end
end
puts nil_or_zero(nil), nil_or_zero(0), nil_or_zero(3)

# a value `when` listed before `when nil` must not call == on nil

#: (Integer?) -> String
def zero_first(v)
  case v
  when 0 then "zero"
  when nil then "nil"
  else "other"
  end
end
puts zero_first(0), zero_first(5), zero_first(nil)

#: (String?) -> String
def str_first(s)
  case s
  when "a" then "A"
  else "other #{s.inspect}"
  end
end
puts str_first("a"), str_first(nil)

# case and if as the value of a block
words = [1, 5, 12].map do |n|
  case n
  when 1 then "one"
  when 5 then "five"
  else "many"
  end
end
puts words.inspect
labels = [1, 5].map { |n| if n > 2 then "big" end }
labels.each { |l| puts l.inspect }

# and/or/not keywords and their precedence
a = true
b = false
puts "and-not" if a and not b
puts "unless-or" unless a || b
puts "not-or" if !(a || b)
puts "neither" unless a or b
puts (a || b && b).inspect
r = (b or a and b)
puts r.inspect

y = z = 5
puts y + z

#: (Integer) -> String
def early(n)
  return "neg" if n < 0
  return "zero" if n.zero?
  unless n > 10
    return "small"
  end
  "big"
end
puts early(-1), early(0), early(5), early(50)

#: (Integer) -> Integer
def abs_val(n) = n < 0 ? -n : n
puts abs_val(-4), abs_val(4)

c = 0
c += 1 if true
c += 10 unless false
c += 100 if false
puts c

# modifiers on a return at top level end the program quietly
xs = [1] #: Array[Integer]
return if xs.empty?
puts "not returned"
return unless xs.empty?
puts "never printed"
