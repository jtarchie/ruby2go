# skip: const_get with a literal path through a non-module constant ("Plugins::VERSION::X") is a compile error reported at line 1; MRI raises TypeError at run time

# rbs_inline: enabled

module Plugins
  VERSION = "1.0"
end

begin
  Object.const_get("Plugins::VERSION::X")
rescue TypeError => e
  puts "TypeError: #{e.message}"
end
