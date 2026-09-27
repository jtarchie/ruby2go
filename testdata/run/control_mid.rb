# rbs_inline: enabled

# a block local first assigned nil joins to T?; only hoisting scope is under test, not a never-assigned zero value
[1, 2].each do |v|
  if v > 5
    mark = nil
  elsif v == 1
    mark = "first"
  end
  puts mark.inspect
end

out = [1, 2, 3].map do |v|
  seen = nil if v > 100
  seen = v * 10 if v.odd?
  seen.inspect
end
puts out.inspect

#: (Array[String]) -> void
def tags(words)
  words.each do |w|
    tag = nil if w.empty?
    tag = "long" if w.size > 3
    puts "#{w}: #{tag.inspect}"
  end
end
tags(["ruby", "go", "rust", "c"])

# a block param shadows an outer local of the same name without assigning it
x = 10
[1, 2].each { |x| puts x }
puts x
ys = [1, 2].map { |x| x * 3 }
puts ys.inspect, x

#: (Array[String]) -> String
def last_seen(items)
  item = "none"
  items.each do |item|
    puts "item #{item}"
  end
  item
end
puts last_seen(["a", "b"])

# break inside a case arm leaves the enclosing loop, not the case
i_break = 0
while i_break < 10
  i_break += 1
  case i_break
  when 3 then break
  end
end
puts i_break

out_break = [] #: Array[String]
[1, 2, 3].each do |x|
  case x
  when 2 then break
  else out_break << x.to_s
  end
end
puts out_break.inspect

vals = [1, "s", nil] #: Array[untyped]
n_break = 0
vals.each do |v|
  n_break += 1
  case v
  when String then break
  end
end
puts n_break

# an overridden message or to_s drives what inspect and rescue see
class CustomMessage < StandardError
  #: () -> String
  def message = "custom"
end

class CustomToS < StandardError
  #: () -> String
  def to_s = "tos"
end

puts CustomMessage.new("x").to_s, CustomMessage.new("x").inspect, CustomMessage.new("x").message
puts CustomToS.new("x").message, CustomToS.new("x").inspect
begin
  raise CustomToS, "zz"
rescue => e_tos
  puts e_tos.message
end

# is_a? narrows a T? local so the typed call accepts it

#: (Integer) -> Integer
def dbl(n) = n * 2

class Animal
  #: () -> String
  def name = "animal"
end

#: (Animal) -> String
def greet(a) = "hi #{a.name}"

x_narrow = 5 #: Integer?
if x_narrow.is_a?(Integer)
  puts dbl(x_narrow)
end
y_narrow = nil #: Integer?
puts(y_narrow.is_a?(Integer) ? dbl(y_narrow) : -1)
pet = Animal.new #: Animal?
puts greet(pet) if pet.is_a?(Animal)

# loop jumps from inside begin/rescue/ensure must still reach the enclosing loop
i_jump = 0
seen_jump = [] #: Array[Integer]
while i_jump < 6
  i_jump += 1
  begin
    next if i_jump == 2
    break if i_jump == 5
    raise "odd" if i_jump.odd?
    seen_jump << i_jump
  rescue
    seen_jump << -i_jump
  end
end
puts seen_jump.inspect, i_jump

out_jump = [] #: Array[Integer]
[1, 2, 3].each do |x|
  begin
    raise "two" if x == 2
    out_jump << x
  rescue
    next
  ensure
    out_jump << 0
  end
  out_jump << 10
end
puts out_jump.inspect

# a closure-style block: `next` inside begin still runs ensure and skips the rest

#: () { (Integer) -> void } -> void
def guarded
  [1, 2, 3].each { |g| yield g }
rescue => e
  puts "guarded caught #{e.message}"
end

guarded do |g|
  begin
    next if g == 2
    puts "in begin #{g}"
  ensure
    puts "ensure #{g}"
  end
  puts "after begin #{g}"
end

guarded do |g|
  begin
    raise "odd" if g.odd?
  rescue
    next
  end
  puts "even #{g}"
end

# nil assigned after a typed assignment joins the local to T?

#: (Integer) -> void
def joined(n)
  y = "a"
  y = nil if n > 5
  puts y.inspect
  if n > 0
    label = "pos"
  else
    label = nil
  end
  puts label.inspect
end
joined(1)
joined(9)
joined(-1)

# rescue and ensure bodies on a value-returning block
ds_rescue = [2, 0, 5] #: Array[Integer]
q_rescue = ds_rescue.map do |d|
  10 / d
rescue ZeroDivisionError
  -1
end
puts q_rescue.inspect
r_rescue = ds_rescue.map { |d| begin; 10 / d; rescue ZeroDivisionError; -2; end }
puts r_rescue.inspect
s_ensure = [2, 5].map do |d|
  10 / d
ensure
  puts "ensure #{d}"
end
puts s_ensure.inspect

# a rescue modifier whose fallback itself raises

#: (Integer) -> Integer
def risky(n)
  raise ArgumentError, "neg" if n < 0
  n
end

begin
  v_wrap = risky(-1) rescue raise(KeyError, "wrapped")
  puts v_wrap
rescue KeyError => e_wrap
  puts e_wrap.message
end
w_risky = risky(2) rescue raise("no")
puts w_risky

# same-named locals in sibling blocks are separate, even with different types
[1, 2].each do |z|
  w = z * 2
  puts w
end
[3].each do |z|
  w = "s#{z}"
  puts w
end

names = ["ann", "bo"] #: Array[String]
names.each { |n| s = n.upcase; puts s }
[4, 5].each { |c| s = c * 2; puts s }

#: () -> void
def later_outer
  labels = [1, 2].map do |z|
    item = z.to_s
    item
  end
  puts labels.inspect
  item = 5
  puts item + 1
end
later_outer

# a value block inside begin, and a method-level ensure around one
ds_begin = [2, 5] #: Array[Integer]
begin
  q_begin = ds_begin.map { |d| 10 / d }
  puts q_begin.inspect
rescue ZeroDivisionError => e_div
  puts e_div.message
end

#: (Array[Integer]) -> Array[Integer]
def halves(list)
  list.map { |d| d / 2 }
ensure
  puts "done"
end
puts halves([4, 6]).inspect

# assignments and calls inside while/until conditions
queue = [1, 2, 3] #: Array[Integer]
guard = 0
while (item = queue.shift)
  guard += 1
  break if guard > 5
  puts item
end
puts queue.inspect, guard

q_while = [1, 2, 5, 1] #: Array[Integer]
guard = 0
while (q_while[0] || 0) < 3
  guard += 1
  break if guard > 5
  puts q_while.shift.inspect
end
puts guard

stack = [4, 0] #: Array[Integer]
guard = 0
until (top = stack.pop).nil?
  guard += 1
  break if guard > 5
  puts top
end
puts guard

# zsuper in an exception initialize forwards the (defaulted) message
class Plain < StandardError
  #: (String) -> void
  def initialize(msg)
    super
  end
end

class DefaultMsg < StandardError
  #: (?String) -> void
  def initialize(msg = "default message")
    super
  end
end

puts Plain.new("x").message
begin
  raise DefaultMsg
rescue DefaultMsg => e_default
  puts e_default.message
end

# ys_next is printed without Array[Integer?]#inspect (see control_bug_next_in_value_block)
xs = [1, 2, 3] #: Array[Integer]
ys_next = xs.map do |x|
  next if x == 2
  x * 10
end
puts ys_next.map { |y| y ? y.to_s : "nil" }.join(",")
picked = xs.select do |x|
  next if x.odd?
  true
end
puts picked.inspect
puts xs.map { next }.inspect
ws = xs.map do |x|
  next if x == 1
  puts x
end
puts ws.inspect
