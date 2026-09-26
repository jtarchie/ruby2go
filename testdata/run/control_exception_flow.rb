# rbs_inline: enabled

class Fixed < StandardError
  #: () -> void
  def initialize
    super("fixed message")
  end
end

class Blank < StandardError
  #: () -> void
  def initialize
    super()
  end
end

class WithCode < StandardError
  attr_reader :code #: Integer

  #: (Integer) -> void
  def initialize(code)
    @code = code
    super("code #{code}")
  end
end

begin
  raise Fixed
rescue Fixed => e
  puts e.message
end
begin
  raise Blank
rescue => e
  puts e.message, e.inspect
end
begin
  raise WithCode.new(42)
rescue WithCode => e
  puts e.code, e.message
end

# an exception raised in ensure replaces the pending one
begin
  begin
    raise ArgumentError, "first"
  ensure
    puts "ensure raising"
    raise KeyError, "from ensure"
  end
rescue => e
  puts "#{e.class}: #{e.message}"
end

# rescue that raises, with ensure in the same method

#: () -> Integer
def rescue_raises
  raise ArgumentError, "a"
rescue ArgumentError
  raise KeyError, "b"
ensure
  puts "ens"
end
begin
  rescue_raises
rescue KeyError => e
  puts e.message
end

# nested rescue inside a rescue body, outer binding still visible
begin
  raise "x"
rescue => e
  begin
    raise ArgumentError, "inner #{e.message}"
  rescue ArgumentError => e2
    puts "nested #{e2.message} outer #{e.message}"
  end
end

# the same name bound in two rescue clauses with different classes

#: (Integer) -> String
def two_bindings(k)
  raise ArgumentError, "arg" if k == 0
  raise KeyError, "key" if k == 1
  "none"
rescue ArgumentError => e
  "A #{e.message}"
rescue KeyError => e
  "K #{e.message}"
end
puts two_bindings(0), two_bindings(1), two_bindings(2)

# exception objects stored in collections and passed through untyped
errors = [] #: Array[StandardError]
[ArgumentError.new("a"), KeyError.new("k")].each { |x| errors << x }
errors.each { |x| puts "#{x.class}: #{x.message}" }
anything = RuntimeError.new("untyped") #: untyped
puts anything.message
begin
  raise errors[1] || StandardError.new("fallback")
rescue KeyError => e
  puts "stored #{e.message}"
end

# rescue matches a superclass in a list with a non-match first
begin
  raise KeyError, "kk"
rescue TypeError, IndexError => e
  puts "list: #{e.class} #{e.is_a?(IndexError)}"
end

# rescue Exception catches StandardError too
begin
  raise "std"
rescue Exception => e
  puts "Exception caught #{e.class}"
end


# raise in a deeply nested call inside a loop inside a method with rescue

#: (Array[Integer]) -> String
def sum_positive(xs)
  total = 0
  xs.each do |x|
    raise ArgumentError, "negative #{x}" if x < 0
    total += x
  end
  "sum #{total}"
rescue ArgumentError => e
  "failed: #{e.message} after #{total}"
end
puts sum_positive([1, 2, 3]), sum_positive([1, -2, 3])

# ensure runs on normal completion inside a loop, every iteration
3.times do |t|
  begin
    puts "try #{t}"
  ensure
    puts "done #{t}"
  end
end

# rescue with no binding and a class list
begin
  raise TypeError, "t"
rescue ArgumentError, TypeError
  puts "no binding"
end


# raise message built from interpolation and a to_s
class Thing
  #: () -> String
  def to_s = "thing"
end
begin
  raise ArgumentError, "bad #{Thing.new}"
rescue => e
  puts e.message
end

# e.message is a String and can be chained
begin
  raise "Mixed Case"
rescue => e
  puts e.message.upcase, e.message.size
end

puts ArgumentError.new("x").class == ArgumentError, KeyError.name

module App
  class Error < StandardError; end
  class Missing < Error; end

  #: (String) -> String
  def self.find(k)
    raise Missing, "no #{k}" if k.empty?
    k
  end
end

begin
  App.find("")
rescue App::Error => e
  puts "#{e.class}: #{e.message} #{e.is_a?(App::Missing)}"
end

# re-raise the same object from a rescue
begin
  begin
    raise KeyError, "orig"
  rescue KeyError => e
    puts "logging #{e.message}"
    raise e
  end
rescue => outer
  puts "outer #{outer.class} #{outer.message}"
end

# an exception raised in initialize escapes new
class Strict
  attr_reader :n #: Integer

  #: (Integer) -> void
  def initialize(n)
    raise ArgumentError, "n must be positive, got #{n}" unless n > 0
    @n = n
  end
end
begin
  Strict.new(-1)
rescue ArgumentError => e
  puts e.message
end
puts Strict.new(2).n

#: (Integer) -> Integer
def depth(n)
  return depth(n - 1) + 1 if n > 0
  raise ArgumentError, "bottom"
rescue ArgumentError
  n * 1000
end
puts depth(3)

#: (bool) -> void
def void_ensure_return(flag)
  puts "body"
  raise "x" if flag
ensure
  puts "ensure"
  return
end
void_ensure_return(false)
void_ensure_return(true)
puts "swallowed"

#: (Integer) -> String
def rescue_then_ensure_return(n)
  raise ArgumentError, "a" if n > 0
  "body"
rescue ArgumentError
  "rescue"
ensure
  puts "ensure #{n}"
end
puts rescue_then_ensure_return(0), rescue_then_ensure_return(1)

# rescue clauses see locals assigned before the raise, even in a loop

#: (Array[Integer]) -> String
def progress(xs)
  done = 0
  xs.each do |x|
    raise ArgumentError, "stop at #{x}" if x < 0
    done += 1
  end
  "all #{done}"
rescue ArgumentError => e
  "#{e.message} after #{done}"
end
puts progress([1, 2]), progress([1, -1, 2])


#: (Array[Integer]) -> Integer
def first_ok(xs)
  xs.each do |x|
    begin
      raise "bad #{x}" if x < 0
      return x * 10
    rescue => e
      puts "skip #{e.message}"
    end
  end
  -1
end
puts first_ok([-1, -2, 3, 4]), first_ok([-5])

#: (Integer) -> String
def case_tail(n)
  raise ArgumentError, "neg" if n < 0
  case n
  when 0 then "zero"
  else "pos"
  end
rescue ArgumentError
  "rescued"
end
puts case_tail(0), case_tail(3), case_tail(-3)

class Cache
  #: () -> void
  def initialize
    @calls = 0
  end

  #: () -> Integer
  def value
    @v ||= compute
  rescue ArgumentError
    -1
  end

  #: () -> Integer
  def compute
    @calls += 1
    raise ArgumentError, "first call fails" if @calls == 1
    @calls * 100
  end
end
cc = Cache.new
puts cc.value, cc.value, cc.value

#: (Integer) -> Integer
def risky(n)
  raise ArgumentError, "neg" if n < 0
  n
end

class Holder
  attr_reader :v #: Integer

  #: (Integer) -> void
  def initialize(n)
    @v = begin
      risky(n)
    rescue ArgumentError
      0
    end
  end
end
puts Holder.new(5).v, Holder.new(-5).v

puts(begin
  risky(-2)
rescue ArgumentError => e
  e.message.size
end)

#: (Array[Integer]) -> Array[String]
def labels(xs)
  xs.map do |x|
    x > 0 ? "p#{x}" : raise(ArgumentError, "bad #{x}")
  end
end
puts labels([1, 2]).inspect
begin
  labels([1, -2])
rescue ArgumentError => e
  puts e.message
end

#: (Array[Integer]) -> Array[String]
def names(xs)
  xs.map do |x|
    case x
    when 1 then "one"
    else raise ArgumentError, "unknown #{x}"
    end
  end
end
puts names([1, 1]).inspect
begin
  names([1, 2])
rescue ArgumentError => e
  puts e.message
end

#: (Array[Integer]) -> Array[Integer]
def divs(xs) = xs.map { |d| 10 / d }
begin
  divs([1, 0])
rescue ZeroDivisionError => e
  puts "from map: #{e.message}"
end

#: () -> Integer
def multi_in_begin
  begin
    a, b = 3, 4
  rescue
    a = 0
    b = 0
  end
  a + b
end
puts multi_in_begin

# rescue modifier on the right of ||=
opt = nil #: Integer?
opt ||= (risky(-1) rescue 5)
puts opt

# ensure sees the loop variable of an enclosing iterator
[1, 2].each do |i|
  begin
    raise "odd" if i.odd?
  rescue
    puts "rescued #{i}"
  ensure
    puts "ensure #{i}"
  end
end

# exceptions from an untyped call
u = [1, 2] #: untyped
begin
  u.fetch(10)
rescue IndexError => e
  puts "untyped fetch: #{e.class}"
end
