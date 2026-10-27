# rbs_inline: enabled

# setbyte writes in place on a String nothing else holds: a fresh local, or an ivar built in initialize (decision 395).

#: (String, String) -> String
def xor(data, key)
  out = data.dup
  i = 0
  while i < out.bytesize
    out.setbyte(i, (data.getbyte(i) || 0) ^ (key.getbyte(i % key.bytesize) || 0))
    i += 1
  end
  out
end

#: (String) -> String
def rot13(text)
  out = text.dup
  out.bytesize.times do |i|
    c = out.getbyte(i) || 0
    if c >= 97 && c <= 122
      out.setbyte(i, ((c - 97 + 13) % 26) + 97)
    elsif c >= 65 && c <= 90
      out.setbyte(i, ((c - 65 + 13) % 26) + 65)
    end
  end
  out
end

class Bitmap
  attr_reader :width #: Integer

  #: (Integer, Integer, Array[[Integer, Integer]]) -> void
  def initialize(width, height, points)
    @width = width
    @pixels = " " * (width * height)
    points.each { |(x, y)| @pixels.setbyte((y * width) + x, 42) }
  end

  #: () -> Array[String]
  def rows = @pixels.scan(/.{#{@width}}/)
end

secret = xor("attack at dawn", "key")
p secret.bytes.first(5)
puts xor(secret, "key")
puts rot13("Hello, World!")
puts rot13(rot13("round trip"))

diamond = [[4, 0], [3, 1], [5, 1], [2, 2], [6, 2], [3, 3], [5, 3], [4, 4]] #: Array[[Integer, Integer]]
Bitmap.new(9, 5, diamond).rows.each { |r| puts "|#{r}|" }
