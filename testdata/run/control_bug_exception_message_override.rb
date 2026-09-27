# rbs_inline: enabled

class CustomMessage < StandardError
  #: () -> String
  def message = "custom"
end

class CustomToS < StandardError
  #: () -> String
  def to_s = "tos"
end

puts CustomMessage.new("x").to_s, CustomMessage.new("x").inspect, CustomMessage.new("x").message
puts CustomToS.new("x").message, CustomToS.new("x").inspect
begin
  raise CustomToS, "zz"
rescue => e
  puts e.message
end
