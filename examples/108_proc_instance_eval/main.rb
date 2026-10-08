# rbs_inline: enabled

# A block stored in an ivar and run later with a different self (#85).

class Runner
  #: () { () -> String } -> void
  def initialize(&blk)
    @blk = blk
  end

  #: () -> String
  def run
    instance_eval(&@blk)
  end

  #: () -> String
  def hello = "hi"
end

puts Runner.new { hello }.run
