# rbs_inline: enabled

# A class body may guard definitions with a condition rb2go knows at compile
# time (#88, #89): a RUBY_VERSION comparison or defined? of a constant or class
# method. The branch MRI would take is compiled; the other is dropped.

module Clock
  def self.label = "clock"
end

class Feature
  if RUBY_VERSION >= "2.5"
    def two_five = "two-five"
  end

  unless RUBY_VERSION < "3.0"
    def three_oh = "three-oh"
  end

  if RUBY_VERSION >= "99.0" && RUBY_VERSION < "1.0"
    def impossible = "impossible"
  elsif defined?(Process::CLOCK_MONOTONIC)
    def now = Process.clock_gettime(Process::CLOCK_MONOTONIC)
  else
    def coarse_now = 0.0
  end

  if defined?(Does::Not::Exist) && Does::Not::Exist.instance_method(:x)
    def gone = "gone"
  end

  if defined?(Clock.label) && !defined?(Clock.missing)
    def clock = Clock.label
  end
end

f = Feature.new
puts f.two_five
puts f.three_oh
puts f.respond_to?(:impossible)
puts f.respond_to?(:gone)
puts f.now > 0
puts f.respond_to?(:coarse_now)
puts f.clock
