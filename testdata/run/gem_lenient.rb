# gem: gem_lenient
# load_path: gem_lenient
# Gem code compiled leniently (decision 410): constants a constant's value assigns, a container ivar reset to nil,
# and a nil in a multiple assignment; matrix's shapes (decision 414).
require "foo"
require "ng"
require "mx"
require "ftpish"
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
p g.sieve(10)
p g.upto_inf(3)
x = Mx.new([[1, 2], [3, 4]])
p x.solve(5..6)
p x.fill
p x.each_with_index.to_a
p x.columns([0, 1])
p Mx.build(2) { |i, j| i * 10 + j }
p x.map.to_a, x.collect(:diagonal).to_a
p x.find_index(4), x.index { |e| e > 2 }, x.find_index(:all, 9)
p x.strict_upper_even?
p Mx.diagonal(1, 2)
p Mx.ramp(4)
p x.twin_solve
p x.dyn_col
f = Ftpish.new
p f.parse("t", "2024.5"), f.parse("t", "2024"), f.parse("z", "q")
p Ftpish::KIND, f.connect, f.get, f.get("x"), f.drain("ab")
w = Ftpish::Wire.new
p f.push(w, "STAT"), w.sent, f.by_name("abc", "upcase")
p f.soft_require, f.forward(1, 2), f.pump, Ftpish::Proto.open("u", {}, false)
