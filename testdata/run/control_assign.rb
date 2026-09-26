# rbs_inline: enabled

calls = [] #: Array[String]

#: (Array[String], String, Integer?) -> Integer?
def trace(log, name, v)
  log << name
  v
end

a = nil #: Integer?
a ||= trace(calls, "first", 1)
a ||= trace(calls, "second", 2)
puts a.inspect, calls.inspect

fresh ||= "made"
puts fresh.inspect

n = 0
n ||= trace(calls, "never", 9) || 0
puts n, calls.inspect

flag = false
flag ||= true
puts flag
t = true
t ||= false
puts t

untyped_vals = [false, nil, 0, "", [], :s] #: Array[untyped]
untyped_vals.each do |u|
  u ||= "was falsy"
  puts u.inspect
end

m = nil #: String?
m ||= nil
puts m.inspect
m ||= "later"
puts m.inspect
m ||= "ignored"
puts m.inspect

class Memo
  #: () -> void
  def initialize
    @hits = 0
  end

  #: () -> Integer
  def hits = @hits

  #: () -> String
  def value
    @value ||= compute
  end

  #: () -> Array[Integer]
  def list
    @list ||= [@hits]
  end

  #: () -> bool
  def ready
    @ready ||= @hits > 0
  end

  private

  #: () -> String
  def compute
    @hits += 1
    "v#{@hits}"
  end
end
memo = Memo.new
puts memo.ready, memo.value, memo.value, memo.hits, memo.list.inspect, memo.ready

cache = nil #: Array[Integer]?
3.times do |i|
  cache ||= []
  cache << i
end
puts cache.inspect

#: (Integer, Integer) -> [Integer, Integer]
def divmod2(x, y) = [x / y, x % y]

#: () -> [String, Integer, bool]
def triple = ["t", 3, true]

q, r = divmod2(17, 5)
puts q.inspect, r.inspect
q, r = divmod2(-7, 2)
puts q.inspect, r.inspect
s, i, b = triple
puts s.inspect, i.inspect, b.inspect

a1, b1 = 1, 2
a1, b1 = b1, a1
puts a1, b1
x, y, z = "x", "y", "z"
x, y, z = y, z, x
puts [x, y, z].inspect

f0, f1 = 0, 1
10.times { f0, f1 = f1, f0 + f1 }
puts f0, f1

arr = [10, 20] #: Array[Integer]
p1, p2, p3 = arr
puts p1.inspect, p2.inspect, p3.inspect
e1, e2 = [] #: Array[String]
puts e1.inspect, e2.inspect
one, two = [5] #: Array[Integer]
puts one.inspect, two.inspect
h1, h2 = 1, 2, 3
puts h1.inspect, h2.inspect

mm = 0
nn = 0
mm, nn = 3, 4
puts mm + nn

class Point
  attr_reader :x #: Integer
  attr_reader :y #: Integer

  #: (Integer, Integer) -> void
  def initialize(x, y)
    @x, @y = x, y
  end

  #: () -> void
  def flip
    @x, @y = @y, @x
  end

  #: () -> String
  def to_s = "(#{@x}, #{@y})"
end
pt = Point.new(1, 2)
pt.flip
puts pt

pairs = [[1, "one"], [2, "two"]] #: Array[[Integer, String]]
pairs.each do |num, word|
  lo, hi = num, num * 100
  puts "#{word}: #{lo}..#{hi}"
end

maybe = "set" #: String?
other = 1
maybe, other = nil, 2
puts maybe.inspect, other

log = [] #: Array[String]

#: (Array[String], String, bool) -> bool
def tb(log, name, v)
  log << name
  v
end

r1 = tb(log, "a", false) && tb(log, "b", true)
r2 = tb(log, "c", true) || tb(log, "d", true)
r3 = tb(log, "e", true) && tb(log, "f", false)
r4 = tb(log, "g", false) || tb(log, "h", false)
puts [r1, r2, r3, r4].inspect, log.inspect

log.clear
r5 = (tb(log, "i", true) and tb(log, "j", true))
r6 = (tb(log, "k", false) or tb(log, "l", true))
r7 = (not tb(log, "m", true))
puts [r5, r6, r7].inspect, log.inspect

none = nil #: Integer?
some = 3 #: Integer?
puts (none || 10).inspect, (some || 10).inspect
puts (none && none + 1).inspect, (some && some + 1).inspect
str = nil #: String?
puts (str || "").inspect, (str && str.size).inspect
str = "abc"
puts (str && str.size).inspect
other_nil = nil #: Integer?
puts (none || other_nil).inspect

yes = true
no = false
puts (no || "fallback").inspect, (yes && "yes").inspect
puts (no && "never").inspect, (yes || "never").inspect
puts (none || false).inspect, (some && "three").inspect, (none && "none").inspect

zero_and = 0 && "zero is truthy"
puts zero_and.inspect
empty_and = "" && :empty_is_truthy
puts empty_and.inspect

xn = nil #: Integer?
yn = nil #: Integer?
zn = 7 #: Integer?
puts (xn || yn || zn).inspect, (zn && zn > 5 && "big").inspect, (xn || yn || 0).inspect

counter = [] #: Array[Integer]

#: (Array[Integer], Integer) -> Integer
def note(c, v)
  c << v
  v
end

one_opt = 1 #: Integer?
got = one_opt || note(counter, 2)
got2 = none || note(counter, 3)
got3 = none && note(counter, 4)
got4 = one_opt && note(counter, 5)
puts got.inspect, got2.inspect, got3.inspect, got4.inspect, counter.inspect

vals = [nil, false, 0, "s"] #: Array[untyped]
vals.each do |v|
  puts (v || "right").inspect + " " + (v && "right").inspect
end

puts (!none).inspect, (!some).inspect, (!!some).inspect, (!yes).inspect, (!!no).inspect

h = { "a" => 1 } #: Hash[String, Integer]
v = h["a"] or raise "missing"
puts v.inspect
begin
  w = h["b"] or raise KeyError, "missing b"
  puts w.inspect
rescue KeyError => e
  puts e.message
end
ok = h.key?("a")
ok or puts "or skipped"
puts "unless-and" unless ok && h.empty?
