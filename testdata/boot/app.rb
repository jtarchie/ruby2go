# load_path: lib
require "demo"
require "demo/text"
require "demo/gemish"

# A computed require in a method is a compile-time no-op (decision 163); under MRI it loads demo/unnamed.
def load_extra(name) = require(name)
load_extra("demo/unnamed")

puts Demo::Thing.new("hi").greet
puts Demo::Lazy.new.go
p Demo::Text.forwarded_values("for=192.0.2.60;proto=http;by=203.0.113.43")
p Demo::Text.forwarded_values(%q{for="_gazonk", For="[2001:db8:cafe::17]:4711"})
p Demo::Text.forwarded_values(nil)
p Demo::Text.first_param(+"for=x")
p Demo::Config::DEFAULTS
g = Demo::Gemish.new
p g.state
p Demo::Gemish::Pair.new("k", 1).to_a
p Demo::Gemish::Part.new("b", "n").label
h = {} #: Hash[String, String]
p g.kind?(h)
h["kind"] = "html"
p g.kind?(h)
p g.joined
p g.first_big([1]) if ENV["RB2GO_NEVER"]
begin
  g.recheck
rescue ArgumentError => e
  p e.message
end
