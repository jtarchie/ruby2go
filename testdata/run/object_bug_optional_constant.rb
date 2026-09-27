# rbs_inline: enabled

NAME = "n" #: String?
COUNT = 3 #: Integer?

module Settings
  LABEL = "l" #: String?
end

puts NAME.inspect, COUNT.inspect, Settings::LABEL.inspect
puts NAME.upcase if NAME
puts (COUNT || 0) + 1
