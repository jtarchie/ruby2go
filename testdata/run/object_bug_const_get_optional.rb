# rbs_inline: enabled

module Settings
  DEFAULT = nil #: String?
end

puts Settings::DEFAULT.inspect, Settings.constants.inspect
puts Settings.const_get(:DEFAULT).inspect
puts Settings.const_get("DEFAULT").nil?.inspect
