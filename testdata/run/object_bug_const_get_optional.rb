# skip: const_get of a constant typed T? that holds nil panics "interface conversion: interface {} is nil, not *main.String" (exit 1); MRI returns nil

# rbs_inline: enabled

module Settings
  DEFAULT = nil #: String?
end

puts Settings::DEFAULT.inspect, Settings.constants.inspect
puts Settings.const_get(:DEFAULT).inspect
puts Settings.const_get("DEFAULT").nil?.inspect
