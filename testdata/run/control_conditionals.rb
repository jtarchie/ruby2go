# rbs_inline: enabled

#: (Integer) -> String
def sign(n)
  if n > 0
    "positive"
  elsif n < 0
    "negative"
  else
    "zero"
  end
end
puts sign(5).inspect, sign(-3).inspect, sign(0).inspect

# an if without else is nil when the condition fails

#: (Integer) -> String?
def only_positive(n)
  if n > 0
    "yes #{n}"
  end
end
puts only_positive(1).inspect, only_positive(0).inspect, only_positive(-1).inspect

#: (Integer) -> String?
def unless_zero(n)
  unless n.zero?
    "nonzero"
  end
end
puts unless_zero(0).inspect, unless_zero(-2).inspect

#: (Integer) -> String
def unless_else(n)
  unless n.even?
    "odd"
  else
    "even"
  end
end
puts unless_else(3), unless_else(0), unless_else(-4)

# long elsif chains stop at the first match

#: (Integer) -> String
def bucket(n)
  if n < 0 then "neg"
  elsif n < 10 then "digit"
  elsif n < 100 then "tens"
  elsif n < 100 then "never"
  else "big"
  end
end
puts [-1, 0, 9, 10, 99, 100, 1_000_000].map { |n| bucket(n) }.inspect

x = 3
puts (x > 2 ? "big" : "small").inspect
puts (x.even? ? 1 : 2).inspect
z = if x == 3 then 30 else 0 end
puts z.inspect
w = unless x == 3 then 1 else 2 end
puts w.inspect
v = x > 100 ? nil : x * 2
puts v.inspect
v2 = x < 100 ? nil : x * 2
puts v2.inspect
nested = x > 1 ? (x > 2 ? "gt2" : "eq2") : "le1"
puts nested.inspect
puts(if x > 1 then "arg if" else "arg else" end)
s = "a" + (x.odd? ? "odd" : "even") + "z"
puts s

puts "mod-if" if x == 3
puts "mod-if never" if x == 4
puts "mod-unless" unless x == 4
puts "mod-unless never" unless x == 3

# Ruby 4: a line starting with && / || continues the condition
teen = x >= 13
  || x == 3
puts teen

#: (Integer) -> String
def grade(n)
  case n
  when 90, 100 then "A"
  when 80 then "B"
  when -1 then "negative one"
  else "F"
  end
end
puts [90, 100, 80, -1, 0, 85].map { |n| grade(n) }.inspect

#: (String) -> Symbol?
def color(s)
  case s
  when "red" then :warm
  when "blue", "green"
    :cool
  when ""
    :empty
  when "ünï"
    :unicode
  end
end
puts color("red").inspect, color("green").inspect, color("").inspect,
  color("ünï").inspect, color("RED").inspect

#: (Symbol) -> Integer
def sym_case(s)
  case s
  when :a then 1
  when :b, :c then 2
  else 0
  end
end
puts sym_case(:a), sym_case(:c), sym_case(:zz)

# when values can be expressions; the subject is evaluated once

#: (Array[String], Integer) -> Integer
def subject(log, v)
  log << "subject"
  v
end
log = [] #: Array[String]
lim = 3
r = case subject(log, 4)
    when lim then "lim"
    when lim + 1 then "lim+1"
    else "other"
    end
puts r, log.inspect

# case on nilable subject with `when nil`

#: (Integer?) -> String
def nil_case(x)
  case x
  when nil then "nil"
  when 0 then "zero"
  else "other #{x.inspect}"
  end
end
puts nil_case(nil), nil_case(0), nil_case(5)

# case with regexps uses Regexp#===

#: (String) -> String
def kind(s)
  case s
  when /\A\d+\z/ then "digits"
  when /\A[a-z]+\z/ then "lower"
  when /é/ then "has é"
  else "other"
  end
end
puts kind("123"), kind("abc"), kind("café"), kind("A1"), kind("")

#: (bool) -> String
def bool_case(b)
  case b
  when true then "T"
  when false then "F"
  else "?"
  end
end
puts bool_case(true), bool_case(false)

# case on classes is a type test, most specific first
class Animal; end
class Dog < Animal; end
class Cat < Animal; end

#: (untyped) -> String
def what(x)
  case x
  when nil then "nil"
  when Integer then "int #{x + 1}"
  when String then "str #{x.upcase}"
  when Dog then "dog"
  when Animal then "animal"
  when Array then "array of #{x.size}"
  when Hash then "hash of #{x.size}"
  else "other"
  end
end
puts what(nil), what(41), what("hi"), what(Dog.new), what(Cat.new),
  what([1, 2]), what({ "a" => 1 }), what(2.5), what(:sym), what(false)

#: (Animal) -> String
def typed(a)
  case a
  when Dog then "D"
  when Cat then "C"
  else "A"
  end
end
puts typed(Dog.new), typed(Cat.new), typed(Animal.new)

# a superclass arm listed first wins

#: (Animal) -> String
def shadowed(a)
  case a
  when Animal then "animal first"
  when Dog then "never"
  else "else"
  end
end
puts shadowed(Dog.new)

# case without a match and without else is nil
three = 3
none = case three
       when 1 then "x"
       end
puts none.inspect

puts(case three when 3 then "three" else "other" end)
puts "n=#{case three when 3 then "III" else "?" end}"

# untyped values in conditions use Ruby truthiness: only nil and false are falsy

#: (bool) -> String
def yn(flag) = flag ? "yes" : "no"

vals = [nil, false, 0, "", [], true, 0.0] #: Array[untyped]
vals.each do |val|
  word = val ? "T" : "F"
  word2 = "F"
  word2 = "T" if val
  word3 = "T"
  word3 = "F" unless val
  puts "#{val.inspect} #{word} #{word2} #{word3} #{yn(val)} #{(!val).inspect}"
end

# narrowing: `if x`, `x && ...`, ternary, and early-exit guards
maybe = 5 #: Integer?
nothing = nil #: Integer?
puts maybe + 1 if maybe
puts nothing + 1 if nothing
puts (maybe ? maybe * 2 : 0), (nothing ? nothing * 2 : 0)
if maybe && maybe > 4
  puts "maybe #{maybe} > 4"
end

#: (String?) -> String
def guarded(s)
  return "none" unless s
  "got #{s.upcase}"
end
puts guarded(nil), guarded("ok")

#: (String?) -> Integer
def guarded_nil(s)
  return -1 if s.nil?
  s.size
end
puts guarded_nil(nil), guarded_nil("four")
