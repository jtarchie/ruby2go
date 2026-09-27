# rbs_inline: enabled

class Version
  include Comparable

  attr_reader :major #: Integer
  attr_reader :minor #: Integer

  #: (Integer, Integer) -> void
  def initialize(major, minor)
    @major = major
    @minor = minor
  end

  def <=>(other)
    c = major <=> other.major
    return c unless c == 0
    minor <=> other.minor
  end

  #: () -> String
  def to_s = "v#{major}.#{minor}"
end

v1 = Version.new(1, 2)
v2 = Version.new(1, 10)
v3 = Version.new(2, 0)
puts (v1 < v2).inspect, (v2 < v1).inspect, (v1 <= v1).inspect, (v3 > v2).inspect, (v1 >= v3).inspect
puts (v1 <=> v2).inspect, (v3 <=> v1).inspect, (v1 <=> Version.new(1, 2)).inspect
puts v2.between?(v1, v3).inspect, v3.between?(v1, v2).inspect, v1.between?(v1, v1).inspect
puts v1.clamp(v2, v3), v3.clamp(v1, v2), v2.clamp(v1, v3)
puts [v3, v1, v2].sort.map(&:to_s).inspect
puts [v3, v1, v2].min.to_s, [v3, v1, v2].max.to_s
