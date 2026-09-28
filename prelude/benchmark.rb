# prelude/benchmark.rb
# rbs_inline: enabled
#
# Benchmark's wall-clock timer. Always defined.

module Benchmark
  #: () { () -> void } -> Float
  def self.realtime
    start = Time.now
    yield
    Time.now - start
  end
end
