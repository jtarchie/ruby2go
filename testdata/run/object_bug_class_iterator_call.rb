# rbs_inline: enabled

module Colors
  #: () { (String) -> void } -> void
  def self.each
    yield "red"
    yield "green"
  end
end

class Counter
  #: (Integer) { (Integer) -> void } -> void
  def self.upto(n)
    i = 0
    while i < n
      yield i
      i += 1
    end
  end
end

Colors.each { |c| puts c }
Counter.upto(3) { |i| puts i }
