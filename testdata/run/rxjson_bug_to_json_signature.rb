# skip: inside Array/Hash to_json, a user to_json not declared (*untyped) is skipped for the to_s JSON: (state = nil) prints "#<OptState>", () prints "noarg" where MRI raises ArgumentError

# rbs_inline: enabled

require "json"

class OptState
  #: (?untyped) -> String
  def to_json(state = nil) = "\"opt\""
end

class NoArg
  #: () -> String
  def to_json = "\"custom\""

  #: () -> String
  def to_s = "noarg"
end

puts OptState.new.to_json, [OptState.new].to_json, JSON.generate({ "o" => OptState.new })
puts NoArg.new.to_json
puts [NoArg.new].to_json
