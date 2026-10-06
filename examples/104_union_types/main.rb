# rbs_inline: enabled

# Union types: `A | B` is a value of one of a few known classes. rb2go keeps
# the member list, so a call on the value is a Go type switch with a typed
# call per member (no runtime method lookup), a method some member lacks is
# a compile error, and `is_a?`/`case` narrow the value to the members left.

class Circle
  #: (Float) -> void
  def initialize(r)
    @r = r
  end

  #: () -> Float
  def area = (3.14159 * @r * @r).round(2)

  #: () -> String
  def name = "circle"
end

class Square
  #: (Integer) -> void
  def initialize(side)
    @side = side
  end

  #: () -> Integer
  def area = @side * @side

  #: () -> String
  def name = "square"
end

# a token is a number, an operator or a word
#: (String) -> (Integer | Symbol | String)
def token(s)
  if s.match?(/\A\d+\z/)
    s.to_i
  elsif %w[+ - *].include?(s)
    s.to_sym
  else
    s
  end
end

# every member has an arm, so the case never falls through to nil
#: (Integer | Symbol | String) -> String
def kind(t)
  case t
  when Integer then "number #{t * 10}"
  when Symbol then "operator #{t.inspect}"
  when String then "word #{t.upcase}"
  end
end

#: (Array[Integer | Symbol | String]) -> Integer
def evaluate(tokens)
  total = 0
  op = :+
  tokens.each do |t|
    next if t.is_a?(String)

    if t.is_a?(Symbol)
      op = t
    elsif op == :+
      total += t
    elsif op == :-
      total -= t
    else
      total *= t
    end
  end
  total
end

shapes = [Circle.new(1.5), Square.new(3)] #: Array[Circle | Square]
shapes.each { |s| puts "#{s.name}: #{s.area}" }
areas = shapes.map { |s| s.area }
p areas
puts areas.sum

tokens = "2 + 40 * 3 hello".split.map { |w| token(w) }
tokens.each { |t| puts kind(t) }
puts evaluate(tokens)
