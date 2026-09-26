# skip: calling an iterator class method directly with a block (Colors.each { ... }) is a compile error "each is an iterator ...; call it as a statement with a block"

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
