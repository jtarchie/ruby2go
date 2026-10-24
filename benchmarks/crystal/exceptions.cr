# Crystal port of benchmarks/exceptions.rb, for the README comparison column.
# Crystal has no catch/throw or retry: catch/throw becomes `return` from a yielding
# block inside a method, retry becomes a loop around begin/rescue.

# raise/rescue/ensure in a hot loop, retry, custom exception classes, catch/throw.

class ParseError < Exception
  getter pos : Int32

  def initialize(msg : String, @pos : Int32)
    super(msg)
  end
end

def parse_digit(s : String) : Int32
  raise ParseError.new("bad #{s}", s.size) unless s.matches?(/\A\d\z/)

  s.to_i
end

def first_hit(i : Int32) : Int32
  10.times { |j| return j if (i + j) % 7 == 0 }
  0
end

ok = 0
bad = 0
ensured = 0
inputs = %w[1 x 3 4 y 6 7 8 z 0]
100_000.times do |i|
  begin
    ok += parse_digit(inputs[i % 10])
  rescue e : ParseError
    bad += e.pos
  ensure
    ensured += 1
  end
end
puts ok, bad, ensured

found = 0
50_000.times do |i|
  found += first_hit(i)
end
puts found

tries = 0
20_000.times do
  attempts = 0
  loop do
    begin
      attempts += 1
      raise ArgumentError.new("again") if attempts < 3

      tries += attempts
      break
    rescue ArgumentError
    end
  end
end
puts tries

v = 0
100_000.times do |i|
  v += begin
    "#{i % 50}x".to_i
  rescue ArgumentError
    1
  end
  v += (i % 50).to_s.to_i
end
puts v
