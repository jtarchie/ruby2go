# rbs_inline: enabled

module Plugins
  VERSION = "1.0"
end

begin
  Object.const_get("Plugins::VERSION::X")
rescue TypeError => e
  puts "TypeError: #{e.message}"
end
