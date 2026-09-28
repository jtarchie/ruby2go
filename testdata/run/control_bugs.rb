# rbs_inline: enabled

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

# block-local params after `;` shadow the outer local instead of assigning it
x_semi = 5
[1, 2].each { |v; x_semi| x_semi = v * 100 }
puts x_semi
total_semi = 0
sums_semi = [3, 4].map do |v; total_semi|
  total_semi = v + 1
  total_semi
end
puts sums_semi.inspect, total_semi

# literal true/false on the left of &&/|| still yields the right value
puts (false && "never").inspect
puts (true || "never").inspect
puts (true && "yes").inspect
puts (false || "fallback").inspect

# case on a literal subject, with and without an else
v_case = case 3
         when 1 then "one"
         when 3 then "three"
         end
puts v_case.inspect
w_case = case "s"
         when "s" then :matched
         else :no
         end
puts w_case.inspect

# a condition's right side lifted out of &&/|| keeps short-circuiting
h_cond = { "a" => 1, "b" => 5 } #: Hash[String, Integer]
if h_cond["c"].nil? && (h_cond["b"] || 0) > 1
  puts "lifted and"
end
if h_cond["a"].nil? || (h_cond["b"] || 0) > 1
  puts "lifted or"
end
ok_cond = true
unless ok_cond && (h_cond["zz"] || 0) > 1
  puts "unless lifted and"
end

# Exception#inspect with an empty message shows the class name
puts ArgumentError.new("").inspect
begin
  raise KeyError, ""
rescue => e_empty
  puts e_empty.inspect, e_empty.message.inspect
end

# Exception#inspect escapes control characters in the message
puts RuntimeError.new("a\nb").inspect
puts ArgumentError.new("tab\there").inspect

# a bare `next` in a value block yields nil for that element
xs_next = [1, 2, 3] #: Array[Integer]
ys_next = xs_next.map do |x|
  next if x == 2
  x * 10
end
puts ys_next.inspect
picked_next = xs_next.select do |x|
  next if x.odd?
  true
end
puts picked_next.inspect

# nil literal on the left of && is nil
x_nil = nil && 5
puts x_nil.inspect
s_nil = "a"
y_nil = nil && s_nil
puts y_nil.inspect

# nil literal on the left of || is the right value
puts (nil || false).inspect
puts (nil || "right").inspect

# ||= with a raising right side only raises when the left is nil
y_raise = 5 #: Integer?
y_raise ||= raise(ArgumentError, "never")
puts y_raise
x_raise = nil #: Integer?
begin
  x_raise ||= raise(KeyError, "x missing")
  puts "unreachable #{x_raise}"
rescue KeyError => e_raise
  puts e_raise.message
end

# `or`/`||` with a void right side
h_or = { "a" => 1 } #: Hash[String, Integer]
ok_or = h_or.key?("zz")
ok_or or puts "or ran"
h_or["b"] || puts("no b")
h_or["a"] || puts("never")

# a rescue binding is nil after a begin that did not raise

#: (bool) -> void
def check(fail)
  begin
    raise ArgumentError, "bad" if fail
  rescue ArgumentError => err
    puts "rescued"
  end
  puts err.inspect
end
check(true)
check(false)

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

# an untyped local takes new literal types and ||= replaces false
v_untyped = 1 #: untyped
puts v_untyped.inspect
v_untyped = "s"
puts v_untyped.inspect
u_untyped = false #: untyped
u_untyped ||= "was false"
puts u_untyped.inspect

# ||= on an untyped hash read fills false and missing alike
h_fill = { "f" => false, "s" => "set" } #: Hash[String, untyped]
%w[f s missing].each do |k|
  v = h_fill[k]
  v ||= "filled"
  puts v.inspect
end

# truthiness of untyped values read from a hash or array
h_truthy = { "t" => true, "f" => false, "n" => nil, "z" => 0 } #: Hash[String, untyped]
%w[t f n z missing].each do |k|
  puts "#{k}: #{h_truthy[k] ? "truthy" : "falsy"}"
  puts "  if" if h_truthy[k]
  v = h_truthy[k]
  puts "  local if" if v
  puts "  nil? #{h_truthy[k].nil?}"
  puts "  && #{(h_truthy[k] && "rhs").inspect}"
end
arr_truthy = [false, nil, 1] #: Array[untyped]
puts "arr[0] truthy" if arr_truthy[0]
puts "arr[1] truthy" if arr_truthy[1]
puts "arr[2] truthy" if arr_truthy[2]

# value types (String, "", Symbol) are always truthy
s_truthy = "x"
puts "string is truthy" if s_truthy
e_truthy = ""
puts(e_truthy ? "empty string is truthy" : "falsy")
sym_truthy = :a
puts "symbol is truthy" if sym_truthy

# next with a value gives the block's value
nv_h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
puts nv_h.select { |k, v| next false if k == "b"; v > 0 }.inspect
nv_xs = [1, 2] #: Array[Integer]
puts nv_xs.map { |x| next x * 2 }.inspect
puts nv_xs.select { |n| next false if n > 1; true }.inspect
