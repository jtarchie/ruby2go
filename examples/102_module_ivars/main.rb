# rbs_inline: enabled

# A module can keep state in instance variables: each object that includes
# it gets its own, as in Ruby. Declare their types in the module body.

module Tally
  # @rbs @counts: Hash[String, Integer]?

  #: (String) -> void
  def record(event)
    counts = @counts || {} #: Hash[String, Integer]
    counts[event] = counts.fetch(event, 0) + 1
    @counts = counts
  end

  #: (String) -> Integer
  def count_of(event) = @counts&.fetch(event, 0) || 0

  #: () -> Integer
  def total = @counts&.values&.sum || 0
end

class Door
  include Tally

  #: (String) -> void
  def initialize(name)
    @name = name
  end

  #: () -> void
  def open = record("open")

  #: () -> void
  def close = record("close")

  #: () -> String
  def summary = "#{@name}: #{count_of("open")} opens, #{total} events"
end

# A subclass shares the module's state through its parent, and its own
# methods can read the module's instance variables directly.
class GarageDoor < Door
  #: () -> Hash[String, Integer]
  def raw = @counts || {}
end

front = Door.new("front")
back = Door.new("back")
front.open
front.close
front.open
back.open

garage = GarageDoor.new("garage")
garage.open
garage.close

puts front.summary, back.summary, garage.summary
p garage.raw
