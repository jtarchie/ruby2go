# rbs_inline: enabled

# a local assigned inside ensure is visible after

#: () -> String
def ens_local
  begin
    a = 1
  ensure
    cleaned = "cleaned #{a}"
  end
  cleaned
end
puts ens_local

# a local first assigned inside a rescue clause is visible after

#: (Integer) -> String
def rescue_local(d)
  begin
    q = 10 / d
  rescue ZeroDivisionError
    note = "div by zero"
    q = 0
  end
  "#{q} #{note.inspect}"
end
puts rescue_local(0)

# locals assigned in while loop bodies accumulate across iterations
i = 0
parts = [] #: Array[String]
while i < 3
  piece = "p#{i}"
  parts << piece
  i += 1
end
puts parts.join(","), piece

# assigning only, never reading, in several places

#: (bool) -> Integer
def writes_only(flag)
  unused = 1
  if flag
    unused2 = "x"
  end
  3.times { |t| inner_unused = t }
  7
end
puts writes_only(true)

# chained assignment and assignment as an expression
a = b = c = 0
a += 1
puts a, b, c
d = (e = 5) + 1
puts d, e
arr = [] #: Array[Integer]
arr << (f = 9)
puts arr.inspect, f

# op-assign on a narrowed optional
maybe = 3 #: Integer?
if maybe
  maybe += 1
  puts maybe
end
puts maybe.inspect

# ||= in a method on a parameter

#: (String?) -> String
def default_name(name)
  name ||= "anon"
  name.upcase
end
puts default_name(nil), default_name("bo")

# ||= returns the value

#: (Integer?) -> Integer
def or_value(v) = (v ||= 42)
puts or_value(nil), or_value(1)

# multiple assignment inside a while loop from a tuple method

#: (Integer) -> [Integer, Integer]
def step(n) = [n / 2, n % 2]
n = 13
bits = [] #: Array[Integer]
while n > 0
  n, bit = step(n)
  bits << bit
end
puts bits.inspect

# swap through ivars and locals mixed
class Pair
  attr_reader :l #: Integer
  attr_reader :r #: Integer

  #: (Integer, Integer) -> void
  def initialize(l, r)
    @l = l
    @r = r
  end

  #: () -> [Integer, Integer]
  def swapped
    lo, hi = @r, @l
    [lo, hi]
  end
end
lo, hi = Pair.new(1, 2).swapped
puts lo, hi

# multiple assignment of a mixed-type literal
name, age, admin = "ann", 30, false
puts name, age, admin
# destructure with an optional element used after a check
first, second = [7] #: Array[Integer]
puts first + 1 if first
puts (second || -1)

# `it` and numbered params
puts [1, 2].map { it * 2 }.inspect
puts [3, 4].map { _1 + 1 }.inspect

# Ruby locals, params and rescue bindings may use Go keywords and predeclared names
type = "t"
range = 1
default = :d
select = [1, 2] #: Array[Integer]
len = select.size
string = "s"
error = ArgumentError.new("e")
new = 5
copy = new + 1
max = [3, 9].max
min = 2
func = true
go = "go"
var = 1
map = { "a" => 1 } #: Hash[String, Integer]
interface = nil #: Integer?
chan = 3
struct = "st"
package = "pkg"
import = "imp"
fallthrough = 1
defer = 2
goto = 3
const = 4
main = "main"
init = "init"
stdout = "out"
any = select.empty?
bool = false
int = 7
append = [1] #: Array[Integer]
append << 2
panic = "p"
recover = "r"
real = 1.5
iota = 0
byte = "b"
rune = "r"
float64 = 2.5
puts [type, range, default, len, string, error.message, new, copy, max, min, func, go, var,
  map.size, interface.inspect, chan, struct, package, import, fallthrough, defer, goto, const,
  main, init, stdout, any, bool, int, append.inspect, panic, recover, real, iota, byte, rune, float64].inspect

#: (String, Integer) -> String
def kw(type, range) = "#{type}#{range}"
puts kw("a", 1)
begin
  raise "x"
rescue => error
  puts error.message
end
[1].each { |type| puts type }
