# rbs_inline: enabled

#: (Integer) -> Integer
def risky(n)
  raise ArgumentError, "neg" if n < 0
  n
end

# a local assigned on every pass of a block is fresh each iteration
[1, 2, 3].each do |v|
  acc = [] #: Array[Integer]
  acc << v
  puts acc.inspect
end
doubled = [1, 2].map do |v|
  tmp = v * 2
  tmp
end
puts doubled.inspect

# an if modifier / ternary with a nil branch inside interpolation is ""
x = 3
puts "a#{"b" if x > 5}c", "a#{"b" if x < 5}c", "n=#{x > 5 ? "big" : nil}."

# method-level rescue that returns nil into a T? result, and `return` from it

#: (Integer) -> Integer?
def safe(n)
  risky(n)
rescue ArgumentError
  nil
end
puts safe(1).inspect, safe(-1).inspect

#: (Integer) -> Integer
def rescue_return(n)
  risky(n)
rescue ArgumentError
  return -1
end
puts rescue_return(2), rescue_return(-2)

#: (Integer) -> String
def cased(n)
  begin
    case n
    when 1 then return "one"
    when 2 then raise ArgumentError, "two"
    end
    "other"
  rescue ArgumentError => e
    "err #{e.message}"
  end
end
puts cased(1), cased(2), cased(3)

# nested rescue modifiers: the outer one catches the inner fallback's raise
v = (risky(-1) rescue risky(-2)) rescue 3
puts v
v2 = (risky(-1) rescue risky(4)) rescue 3
puts v2

# begin/ensure as a value keeps the body's value
y = begin
  5
ensure
  puts "y ensure"
end
puts y

# a rescued exception can be the subject of a type case
[KeyError.new("k"), ArgumentError.new("a"), IOError.new("io")].each do |ex|
  begin
    raise ex
  rescue => e
    kind = case e
           when KeyError then "key"
           when ArgumentError then "arg"
           else "other #{e.class}"
           end
    puts kind
  end
end

# exceptions print their message through puts, print, interpolation and untyped
class AppError < StandardError; end
begin
  raise AppError, "boom"
rescue => e
  puts e
  puts "Error: #{e}"
  print e, "\n"
end
err = KeyError.new("k") #: untyped
puts err, "u: #{err}", ArgumentError.new, "#{RuntimeError.new}"
list = [ArgumentError.new("a1"), TypeError.new("t1")] #: Array[StandardError]
puts list.map(&:message).inspect, list.map { |l| l.class.name }.inspect, list.inspect
module App
  class Missing < StandardError; end
end
puts App::Missing.new("x").inspect, App::Missing.new.message

# messages built from nil optionals, and used as hash keys
none = nil #: Integer?
begin
  raise ArgumentError, "value was #{none.inspect}"
rescue => e
  puts e.message
end
begin
  raise "count: #{none || 0}"
rescue RuntimeError => e
  puts e.message, e.class
end
msgs = {} #: Hash[String, Integer]
[ArgumentError.new("a"), KeyError.new("a"), TypeError.new("b")].each do |ex|
  msgs[ex.message] = (msgs[ex.message] || 0) + 1
end
puts msgs.inspect

# rescue by a module the exception class includes
module Tag; end
class TaggedError < StandardError
  include Tag
end
begin
  raise TaggedError, "t"
rescue Tag
  puts "rescued by module"
end

# while as the last statement of a value method, a void method and a block

#: (Integer) -> String
def tail_while(n)
  i = 0
  while i < n
    i += 1
  end
  "done #{i}"
end
puts tail_while(3)

#: () -> void
def void_while
  k = 0
  while k < 2
    puts "k#{k}"
    k += 1
  end
end
void_while
[1].each do |z|
  w = 0
  w += 1 while w < z
  puts w
end
a = 0
while a < 2 do a += 1 end
puts a
b = 10
until b < 5 || b.even? && b < 8
  b -= 1
end
puts b

# break from a while inside a begin body, and a loop inside ensure
begin
  n = 0
  while true
    n += 1
    break if n == 3
  end
  puts n
rescue => e
  puts e.message
end
begin
  puts "body"
ensure
  c = 0
  while c < 2
    c += 1
    next if c == 1
    puts "ensure loop #{c}"
  end
end

# break inside an iterator in a rescue clause
begin
  raise "x"
rescue => e
  [1, 2, 3].each do |q|
    break if q == 2
    puts "#{e.message}#{q}"
  end
end

# retry by hand: an until loop around begin/rescue
attempts = 0
done = false
until done
  begin
    attempts += 1
    raise "flaky" if attempts < 3
    done = true
  rescue
    puts "retrying #{attempts}"
  end
end
puts attempts

# conditions that need lifted statements outside && / ||
h = { "a" => 1, "b" => 5 } #: Hash[String, Integer]

#: (Hash[String, Integer], String) -> String
def probe(h, k)
  if k == "x"
    "x"
  elsif (m = h[k])
    "found #{m}"
  else
    "missing"
  end
end
puts probe(h, "x"), probe(h, "a"), probe(h, "zz")
puts((h["q"] || 0) > 0 ? "pos" : "zero")
unless (h["b"] || 0) > 100
  puts "unless lifted"
end
if (h["b"] || 0) > 1 && x > 2
  puts "lifted left"
end

# ! and not on values that are never nil
s = "s"
puts (!x).inspect, (not x).inspect, (!s).inspect, (!!s).inspect

#: (Integer) -> String
def label(n)
  case n
  when 1
    puts "matched one"
    "one"
  when 2, 3
    t = n * 10
    "few #{t}"
  else
    "many"
  end
end
puts label(1), label(3), label(9)
sym = :b
res = case sym
      when :a then 1
      when :b then 2
      end
puts res.inspect
res2 = case sym
       when :zz then 1
       end
puts res2.inspect

# ||= in a while loop, and ||= from another optional
got = nil #: String?
i = 0
while i < 3
  got ||= "set at #{i}"
  i += 1
end
puts got
o1 = nil #: Integer?
o2 = nil #: Integer?
o1 ||= o2
puts o1.inspect
o2 = 4
o1 ||= o2
puts o1.inspect

# && / || chains mixing optionals, Booleans and interpolation
some = 3 #: Integer?
puts ((some && "x") || "y").inspect, ((none && "x") || "y").inspect
name = nil #: String?
puts "hi #{name || "anon"}"
big = some && some > 2
big2 = none && none > 2
puts big.inspect, big2.inspect
flag = false
puts (flag || some).inspect, (flag || none).inspect

# multiple assignment from an Array-returning method and from a pair

#: () -> Array[String]
def parts = ["x", "y", "z"]
p1, p2 = parts
puts p1.inspect, p2.inspect
pair = h.to_a[0] || ["none", 0]
kk, vv = pair
puts kk, vv
arr = [1, 2] #: Array[Integer]
f0, f1 = arr[1], arr[0]
puts [f0, f1].inspect

# next/break guards narrow the block parameter for the rest of the block
opts = [1, nil, 3] #: Array[Integer?]
opts.each do |o|
  next unless o
  puts o + 1
end
opts.each do |o|
  next if o.nil?
  puts o * 2
end

#: (Array[Integer?]) -> Integer
def total_until_nil(xs)
  sum = 0
  xs.each do |x|
    break unless x
    sum += x
  end
  sum
end
puts total_until_nil([1, 2, nil, 4])

# break in an iterator nested in a while leaves only the iterator
laps = 0
while true
  [1, 2].each { |q| break if q == 1 }
  laps += 1
  break if laps > 1
end
puts laps

# decision 20: attribute reads on self narrow like locals
class Box
  attr_reader :v #: Integer?

  #: (Integer?) -> void
  def initialize(v)
    @v = v
  end

  #: () -> String
  def show
    return "empty" unless v
    "v=#{v + 1}"
  end

  #: () -> String
  def show2 = v ? "v=#{v * 2}" : "none"

  #: () -> String
  def show3
    if v && v > 3
      "big #{v}"
    else
      "small"
    end
  end
end
puts Box.new(1).show, Box.new(nil).show, Box.new(2).show2, Box.new(nil).show2, Box.new(5).show3, Box.new(nil).show3

# rescue/ensure in initialize, in a class method and in a module function
class Parser
  attr_reader :ok #: bool

  #: (String) -> void
  def initialize(s)
    @ok = true
    raise ArgumentError, "empty" if s.empty?
  rescue ArgumentError
    @ok = false
  end

  #: (String) -> Integer
  def self.parse(s)
    raise ArgumentError, "bad" unless s == "1"
    1
  rescue ArgumentError => e
    puts "class method rescued #{e.message}"
    0
  ensure
    puts "class method ensure"
  end
end
puts Parser.new("x").ok, Parser.new("").ok
puts Parser.parse("1"), Parser.parse("2")

module Util
  #: (Integer) -> String
  def self.check(n)
    return "neg" if n < 0
    raise KeyError, "zero" if n.zero?
    "pos"
  rescue KeyError => e
    "rescued #{e.message}"
  end
end
puts Util.check(-1), Util.check(0), Util.check(1)

# rescue modifiers as conditions and as the left of ||
puts "cond ok" if (risky(1) rescue nil)
puts "cond never" if (risky(-1) rescue nil)
puts((risky(-1) rescue false) ? "t" : "f")
rx = (risky(2) rescue nil) || 9
ry = (risky(-2) rescue nil) || 9
puts rx, ry

# an iterator that yields inside begin/ensure runs the ensure after each block call

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
each_logged { |lv| puts "v#{lv}" }

# value `when`s on untyped and optional subjects, mixed with `when nil`

#: (untyped) -> String
def classify(v)
  case v
  when 1 then "one"
  when "a", :b then "a or b"
  when nil then "nil"
  when 2.5 then "float"
  else "other"
  end
end
puts classify(1), classify("a"), classify(:b), classify(nil), classify(2.5), classify([1]), classify(false)

#: (String?) -> String
def opt_str(s)
  case s
  when nil then "nil"
  when "x", "y" then "xy"
  else "other"
  end
end
puts opt_str(nil), opt_str("y"), opt_str("z")

# rescue/ensure in inherited, overriding (super inside rescue) and mixed-in methods
class Base
  #: () -> String
  def run = raise(NotImplementedError, "abstract run")

  #: () -> String
  def safe_run
    run
  rescue NotImplementedError => e
    "base caught #{e.message}"
  ensure
    puts "base ensure #{self.class}"
  end
end

class Impl < Base
  #: () -> String
  def run
    super
  rescue NotImplementedError => e
    "impl fallback: #{e.message}"
  end
end

class Plain < Base; end

module Retrying
  #: (Integer) -> String
  def attempt(n)
    raise ArgumentError, "attempt #{n} failed" if n < 2
    "attempt #{n} ok"
  rescue ArgumentError => e
    "#{worker_name}: #{e.message}"
  end
end

class Worker
  include Retrying

  #: () -> String
  def worker_name = "worker"
end

puts Impl.new.run, Impl.new.safe_run, Plain.new.safe_run
wk = Worker.new
puts wk.attempt(1), wk.attempt(2)
