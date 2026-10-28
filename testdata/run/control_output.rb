# rbs_inline: enabled

# Checks that stay print-and-compare; everything else from the control_* files
# lives in testdata/test/control_test.rb.

# a top-level constant reading a top-level local runs where it stands (decision 296)
ctl_out_dir = File.dirname(__FILE__)
CTL_OUT_BASE = File.basename(ctl_out_dir).freeze
puts CTL_OUT_BASE

# `puts` as the void right side of &&/||/and/or and of a rescue modifier.
# A user-defined void method there does not compile yet.

# `and`/`&&` with a void right side, on typed, optional and untyped lefts
h_and = { "a" => 1 } #: Hash[String, Integer]
ok_and = h_and.key?("a")
ok_and and puts "and ran"
ok_and && puts("&& ran")
h_and["a"] && puts("has a")
h_and["b"] && puts("never")
u_and = nil #: untyped
u_and || puts("untyped || ran")
w_and = 1 #: untyped
w_and && puts("untyped && ran")

# `or`/`||` with a void right side
h_or = { "a" => 1 } #: Hash[String, Integer]
ok_or = h_or.key?("zz")
ok_or or puts "or ran"
h_or["b"] || puts("no b")
h_or["a"] || puts("never")
ok_skip = h_or.key?("a")
ok_skip or puts "or skipped"

# a rescue modifier with a void fallback runs only on raise

#: (Integer) -> Integer
def risky(n)
  raise ArgumentError, "neg" if n < 0
  n
end

begin
  risky(-1)
rescue StandardError
  puts("statement fallback")
end
begin
  risky(1)
rescue StandardError
  puts("not printed")
end
log_void = [] #: Array[String]
begin
  risky(-2)
rescue StandardError
  log_void.clear
end
puts "done"

# exceptions print their message through puts, print and untyped
class AppError < StandardError; end
begin
  raise AppError, "boom"
rescue StandardError => e
  puts e
  print e, "\n"
end
err = KeyError.new("k") #: untyped
puts err, ArgumentError.new

# top-level locals may use Go keywords and predeclared names (main, init, ...)
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
begin
  raise "x"
rescue StandardError => error
  puts error.message
end
[1].each { |type| puts type }

# Kernel#display and putc (ruby/spec core/kernel, #49)
class OutDisplayed
  #: () -> String
  def to_s = "shown"
end
p putc(65), putc("hi"), putc(10)
1.display
[1, 2].display
:s.display
OutDisplayed.new.display
p "x".display
$stdout.putc("!")
$stdout.putc(321)
puts

# modifiers on a return at top level end the program quietly (keep last)
# main's own ivars: top-level statements and their blocks share them, a def's self is another object (decision 362)
@main_arr = [0, 4, 7, 10, 12]
puts @main_arr.bsearch_index { |x| x >= 4 }
@main_h = {a: 1}
[1, 2].each { |i| @main_h[:"k#{i}"] = i }
p @main_h
@main_n ||= 5
@main_n += 1
main_a = 1
@main_b = 2
p [main_a, @main_b, @main_n]
p [defined?(@main_arr), defined?(@main_never), @main_never]

# a constant assigned again: MRI's warning, then the new value; one never defined raises where it is read (decision 363)
CTRL_AGAIN = 1
CTRL_AGAIN = 2
p CTRL_AGAIN
def ctrl_dead = CtrlMissing.record(:x)
begin
  ctrl_dead
rescue NameError => e
  puts e.message
end
def ctrl_dead_lit = [CtrlMissing2, 1] # inside a literal too: it raises where it is read
begin
  ctrl_dead_lit
rescue NameError => e
  puts e.message
end
# a top-level def hides Kernel's method of the same name (decision 363)
def test(i) = "mine #{i}"
puts test(3)

# main's public/private, `(a; b)`, loops as values, `a, b = 1`, `def obj.m` on a fresh object, `case nil` (decision 364)
def ctrl_pub = 1
p ctrl_pub
p((ctrl_a = 1; ctrl_a + 1))
p(())
ctrl_w = (break 7 while true)
p ctrl_w
ctrl_wa = (break *[1, 2] while true)
p ctrl_wa
ctrl_i = 0
p((ctrl_i += 1 until ctrl_i > 2))
ctrl_m, ctrl_n = 1
p [ctrl_m, ctrl_n]
ctrl_o = Object.new
def ctrl_o.to_int = 2
p [1, 2, 3][ctrl_o]
ctrl_e = Object.new
def ctrl_e.each
  yield 10
  yield 20
end
ctrl_e.each { |v| p v }
ctrl_c = case nil
         when *[] then "foo"
         end
p ctrl_c

xs = [1] #: Array[Integer]
return if xs.empty?
puts "not returned"
return unless xs.empty?
puts "never printed"
