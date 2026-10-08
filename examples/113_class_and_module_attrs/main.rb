# rbs_inline: enabled

# attr_* inside `class << self` declares class-level accessors (one value per
# class object), and attr_* in a module gives every includer the accessor over
# the module's own instance variable.

class Config
  class << self
    attr_accessor :limit #: Integer?
  end

  self.limit = 3
end

class ChildConfig < Config
end

module Named
  attr_reader :name #: String
  attr_accessor :tag #: Symbol?

  #: (String) -> void
  def initialize(name)
    @name = name
    @tag = nil
  end
end

class Robot
  include Named

  #: () -> String
  def to_s = "#{name} (#{tag.inspect})"
end

puts Config.limit
puts ChildConfig.limit.inspect
ChildConfig.limit = 9
puts ChildConfig.limit
puts Config.limit

r = Robot.new("r2")
r.tag = :droid
puts r
