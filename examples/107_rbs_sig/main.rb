# A class need not carry rbs-inline `#:` comments: a `.rbs` file beside the
# source supplies its signatures (docs/design.md decision 160). `sig/` sits
# next to this file; inline annotations would win over it per symbol.

require_relative "greeter"

g = Greeter.new("world")
puts g.greet
g.name = "ruby"
puts g.greet
