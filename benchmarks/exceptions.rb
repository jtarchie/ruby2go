# rbs_inline: enabled

# raise/rescue/ensure in a hot loop, retry, custom exception classes, catch/throw.

class ParseError < StandardError
  attr_reader :pos #: Integer

  #: (String msg, Integer pos) -> void
  def initialize(msg, pos)
    super(msg)
    @pos = pos
  end
end

#: (String s) -> Integer
def parse_digit(s)
  raise ParseError.new("bad #{s}", s.length) unless s.match?(/\A\d\z/)

  s.to_i
end

ok = 0
bad = 0
ensured = 0
inputs = %w[1 x 3 4 y 6 7 8 z 0]
100_000.times do |i|

  ok += parse_digit(inputs[i % 10] || "")
rescue ParseError => e
  bad += e.pos
ensure
  ensured += 1

end
puts ok, bad, ensured

found = 0
50_000.times do |i|
  found += catch(:done) do
    10.times { |j| throw :done, j if ((i + j) % 7).zero? }
    0
  end
end
puts found

tries = 0
20_000.times do
  attempts = 0
  begin
    attempts += 1
    raise ArgumentError, "again" if attempts < 3

    tries += attempts
  rescue ArgumentError
    retry
  end
end
puts tries

v = 0
100_000.times do |i|
  v += begin
    Integer("#{i % 50}x")
  rescue ArgumentError
    1
  end
  v += Integer((i % 50).to_s)
end
puts v
