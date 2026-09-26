# rbs_inline: enabled

class Shape
  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end
end

class Circle < Shape
  attr_reader :radius #: Integer

  #: (Integer) -> void
  def initialize(radius)
    super("circle")
    @radius = radius
  end

  #: () -> Integer
  def diameter = radius * 2
end

class Square < Shape
  attr_reader :side #: Integer

  #: (Integer) -> void
  def initialize(side)
    super("square")
    @side = side
  end
end

class Node
  attr_reader :value #: Integer
  attr_reader :next_node #: Node?
  attr_reader :shape #: Shape

  #: (Integer, Node?, Shape) -> void
  def initialize(value, next_node, shape)
    @value = value
    @next_node = next_node
    @shape = shape
  end

  #: () -> String
  def tail_value
    if next_node
      "next=#{next_node.value}"
    else
      "last"
    end
  end

  #: () -> String
  def guarded
    return "no next" unless next_node
    "guarded=#{next_node.value + 100}"
  end

  #: () -> String
  def shape_info
    if shape.is_a?(Circle)
      "circle d=#{shape.diameter}"
    elsif shape.is_a?(Square)
      "square s=#{shape.side}"
    else
      "shape #{shape.name}"
    end
  end
end

#: (Integer) -> String?
def word(n) = n.positive? ? "w" * n : nil

#: (String?) -> Integer
def len_if(s)
  if s
    s.size
  else
    -1
  end
end

#: (String?) -> Integer
def len_unless(s)
  return -1 unless s
  s.size
end

#: (String?) -> Integer
def len_nil_guard(s)
  return -2 if s.nil?
  s.size + 1000
end

#: (String?) -> Integer
def len_bang_guard(s)
  return -3 if !s
  s.size + 2000
end

#: (String?) -> Integer
def len_raise_guard(s)
  raise ArgumentError, "need a string" unless s
  s.size
end

#: (String?, String?) -> String
def both(a, b)
  if a && b
    a + b
  elsif a
    "only a=#{a.upcase}"
  elsif b
    "only b=#{b.upcase}"
  else
    "neither"
  end
end

#: (String?) -> String
def long_word(s)
  if s && s.size > 3
    "long #{s}"
  else
    "short or nil"
  end
end

#: (Shape) -> String
def area_of(s)
  return "#{s.name}: #{s.radius * s.radius * 3}" if s.is_a?(Circle)
  return "#{s.name}: #{s.side * s.side}" if s.kind_of?(Square)
  "#{s.name}: ?"
end

#: (Shape) -> Integer
def diameter_guard(s)
  return -1 unless s.is_a?(Circle)
  s.diameter
end

#: (Shape?) -> String
def opt_shape(s)
  if s.is_a?(Circle)
    "opt circle #{s.radius}"
  elsif s
    "opt #{s.name}"
  else
    "opt nil"
  end
end

#: (untyped) -> String
def untyped_narrow(v)
  if v.is_a?(String)
    "string of #{v.size}: #{v.upcase}"
  elsif v.is_a?(Integer)
    "integer doubled #{v * 2}"
  elsif v.is_a?(Array)
    "array of #{v.size}"
  elsif v.is_a?(Hash)
    "hash of #{v.size}"
  elsif v.is_a?(Float)
    "float #{v * 2}"
  elsif v.nil?
    "nil"
  else
    "other"
  end
end

#: (String) -> Integer
def strict_len(s) = s.size

#: (Array[String?]) -> Integer
def total_len(ws)
  sum = 0
  ws.each do |w|
    next if w.nil?
    sum += strict_len(w)
  end
  sum
end

#: (Array[String?]) -> Integer
def first_len(ws)
  ws.each do |w|
    return strict_len(w) if w
  end
  -1
end

#: (String?, String?) -> Integer
def both_or(a, b)
  return -1 unless a && b
  strict_len(a) + strict_len(b)
end

#: (String?) -> String
def triple(q)
  if q && strict_len(q) > 2 && q.start_with?("w")
    "long w-word #{strict_len(q)}"
  else
    "no"
  end
end

#: (untyped) -> String
def unwrap(v)
  depth = 0
  while v.is_a?(Array)
    v = v[0]
    depth += 1
  end
  "#{depth}:#{v.inspect}"
end

#: (untyped) -> String
def retype(v)
  if v.is_a?(String)
    n = v.size
    v = n * 2
    return "was a string, now #{v.inspect} #{v.is_a?(Integer)}"
  end
  "not a string"
end

puts "-- if / unless / guards on T?"
[word(3), word(0), word(1)].each do |w|
  puts len_if(w), len_unless(w), len_nil_guard(w), len_bang_guard(w)
end
puts len_if(""), len_unless(""), len_nil_guard(""), len_bang_guard("")
puts len_raise_guard("abc")
begin
  len_raise_guard(nil)
rescue ArgumentError => e
  puts "ArgumentError: #{e.message}"
end

puts "-- && and elsif chains"
puts both("x", "y"), both("x", nil), both(nil, "y"), both(nil, nil), both("", "")
puts long_word("abcd"), long_word("abc"), long_word(nil), long_word("日本語です")

puts "-- is_a? / kind_of? narrow struct classes"
shapes = [Circle.new(2), Square.new(3), Shape.new("blob")] #: Array[Shape]
shapes.each { |s| puts area_of(s), diameter_guard(s) }
puts opt_shape(Circle.new(5)), opt_shape(Square.new(1)), opt_shape(nil)

puts "-- attribute reads on self narrow"
list = Node.new(1, Node.new(2, nil, Square.new(4)), Circle.new(7))
puts list.tail_value, list.next_node&.tail_value.inspect
puts list.guarded, list.next_node&.guarded.inspect
puts list.shape_info, list.next_node&.shape_info.inspect, Node.new(0, nil, Shape.new("x")).shape_info

puts "-- is_a? narrows untyped"
vals = ["héllo", 21, [1, 2, 3], { "k" => 1 }, 2.5, nil, :sym, true] #: Array[untyped]
vals.each { |v| puts untyped_narrow(v) }

puts "-- while narrows the loop variable"
cur = list #: Node?
total = 0
while cur
  total += cur.value
  cur = cur.next_node
end
puts total, cur.inspect

puts "-- reassigning drops the narrowing"
s = word(2)
if s
  puts s.size
  s = word(0)
  puts s.inspect
  s = word(4)
  puts s.inspect
end
t = word(1)
if t
  t = nil
  puts t.inspect
end

puts "-- ||= makes the value usable"
m = word(0)
m ||= "fallback"
puts m.size, m.inspect

puts "-- ternary and modifier forms"
[word(2), nil].each do |w|
  puts(w ? w.size : 0)
  puts w.size if w
  puts "none" unless w
end

puts "-- narrowing holds inside a block in the branch"
x = word(2)
if x
  [1, 2].each { |i| puts x.size + i }
end

puts "-- next unless in a loop"
[word(1), nil, word(3)].each do |w|
  next unless w
  puts w.upcase
end
puts "-- guards inside blocks, && guards, && chains"
puts total_len([word(1), nil, word(3)]), total_len([]), first_len([nil, word(2)]), first_len([nil])
puts both_or("a", "bc"), both_or(nil, "x"), both_or("x", nil), both_or(nil, nil)
puts triple(word(4)), triple(word(2)), triple(nil), triple("abc")
y = word(3)
puts(y ? strict_len(y) : 0)
puts "-- is_a? narrowing in while, and reassigning an untyped local inside the branch"
puts unwrap([[[1]]]), unwrap(5), unwrap([[nil]]), unwrap([])
puts retype("abc"), retype(1)
puts "-- block parameters narrow like locals"
opt_words = [word(1), nil, word(3)] #: Array[String?]
opt_words.each { |ow| puts strict_len(ow) if ow }
puts opt_words.map { |ow| ow ? strict_len(ow) : 0 }.inspect
pairs = [["a", word(2)], ["b", nil]] #: Array[[String, String?]]
pairs.each do |k, pv|
  next unless pv
  puts "#{k}=#{strict_len(pv)}"
end
puts "done"
