# rbs_inline: enabled

class Memo
  # @rbs @last: Integer?

  #: () -> void
  def initialize
    @last = nil
  end

  #: (Integer) -> Integer
  def store(n)
    @last = n * 2
  end

  #: () -> Integer?
  def last = @last
end

m = Memo.new
puts m.last.inspect
puts m.store(21).inspect
puts m.last.inspect
