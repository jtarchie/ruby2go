# gem: gem_lenient
# load_path: gem_lenient
# Gem code compiled leniently (decision 410): constants a constant's value assigns, a container ivar reset to nil,
# and a nil in a multiple assignment.
require "foo"
require "ng"
p Foo::ORDERINGS
p Foo::FLAGS
p Foo::RETURN_IN_ORDER
p Foo.ordered?
g = Ng.new
p g.multi
p g.names, g.args
g.reset
p g.names, g.args
p g.multi
