# rbs_inline: enabled

# Required by a, then named again by the loader: Ruby loads it once.
puts "b: loading"

class Greeter
  #: () -> String
  def greet = "hello from b"
end
