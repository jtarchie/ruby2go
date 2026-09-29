# rbs_inline: enabled

# Checks that stay print-and-compare; everything else from the control_* files
# lives in testdata/test/control_test.rb.

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

risky(-1) rescue puts("statement fallback")
risky(1) rescue puts("not printed")
log_void = [] #: Array[String]
risky(-2) rescue log_void.clear
puts "done"

# exceptions print their message through puts, print and untyped
class AppError < StandardError; end
begin
  raise AppError, "boom"
rescue => e
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
rescue => error
  puts error.message
end
[1].each { |type| puts type }

# modifiers on a return at top level end the program quietly (keep last)
xs = [1] #: Array[Integer]
return if xs.empty?
puts "not returned"
return unless xs.empty?
puts "never printed"
