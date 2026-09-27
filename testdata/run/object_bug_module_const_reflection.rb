# rbs_inline: enabled

module Config
  LIMIT = 5
end

class Uses
  include Config

  OWN = 1
end

puts Uses.constants.sort.inspect
puts Uses.const_defined?(:LIMIT).inspect
puts Uses.const_get(:LIMIT).inspect
