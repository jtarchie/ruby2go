# rbs_inline: enabled

# Loaded first: require_relative runs b where it stands, so b's output comes
# between a's two lines, and the loader's own require of b later is a no-op.
puts "a: before"
require_relative "b"
puts "a: after, #{Greeter.new.greet}"
