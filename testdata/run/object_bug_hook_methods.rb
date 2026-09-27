# rbs_inline: enabled

class Plugin
  #: (untyped) -> void
  def self.inherited(sub)
    puts "inherited by #{sub}"
  end
end

class Alpha < Plugin
end

module Tracked
  #: (untyped) -> void
  def self.included(base)
    puts "included in #{base}"
  end

  #: (untyped) -> void
  def self.extended(base)
    puts "extended #{base}"
  end
end

class Thing
  include Tracked
end

class Other
  extend Tracked
end

puts "main"
