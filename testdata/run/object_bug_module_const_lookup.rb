# skip: constants of an included module are not visible through the includer: LIMIT inside the class and Uses::LIMIT are "uninitialized constant"; decision 17 says the ancestors are searched

# rbs_inline: enabled

module Config
  LIMIT = 5
end

class Uses
  include Config

  #: () -> Integer
  def lim = LIMIT
end

class Sub < Uses
  #: () -> Integer
  def twice = LIMIT * 2
end

puts Uses.new.lim.inspect, Sub.new.twice.inspect
puts Uses::LIMIT.inspect, Sub::LIMIT.inspect
