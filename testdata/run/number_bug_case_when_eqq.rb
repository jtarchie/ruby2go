# rbs_inline: enabled

class Div
  #: (Integer) -> void
  def initialize(n)
    @n = n
  end

  #: (Integer) -> bool
  def ===(x) = x % @n == 0
end

#: (untyped) -> String
def kind(v)
  case v
  when Integer then "int"
  when Float then "float"
  when true then "true"
  else "other"
  end
end

#: (Integer) -> String
def fizz(i)
  case i
  when Div.new(15) then "FizzBuzz"
  when Div.new(3) then "Fizz"
  when Div.new(5) then "Buzz"
  else i.to_s
  end
end

#: (Integer) -> String
def sized(n)
  case n
  when 0 then "zero"
  when Integer then "int"
  else "?"
  end
end

puts kind(1), kind(2.5), kind(true), kind("s")
puts [1, 3, 5, 15].map { |i| fizz(i) }.inspect
puts sized(0), sized(7)
