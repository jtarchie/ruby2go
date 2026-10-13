# load_path: lib
require "demo"
require "demo/text"
require "demo/gemish"
require "demo/cli"

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
p g.setup_optional(false)
begin
  g.setup_optional(true)
rescue NameError => e
  p e.class, e.message
end
p g.never_set_or(7), Demo::Gemish.cached
p g.dyn_send(g, :state), g.dyn_send(Demo::GemTab.empty, :kind), g.dyn_send(Demo::Lenient, :mixed, false), g.dyn_send(g, :frozen?)
p g.dyn_responds(g, :joined), g.dyn_responds(g, :nope), g.dyn_responds(Demo::GemTab.empty, :kind)
p g.json_kids('{"kids":[{"name":"x","age":5},{"name":"y","age":2}]}')
p g.json_find('{"sigs":[{"h":{"kid":"a"}},{"h":{"kid":"b"},"v":1}]}'), g.json_find("{}")
p g.json_big('{"a":1,"b":2}')
p g.tab_for("x").kind, g.tab_for("")
p g.synced
p g.remaining(3), g.remaining(0.5)
$stdout.ohai("extended")
cli = Demo::CliParser.new.parse(["json"])
p cli.json?, cli.sizes?

p DemoCli::InfoCmd.new.run
ENV["DEMO_X_TOKEN"] = "s"
p ENV.sensitive?("A_TOKEN"), ENV.sensitive_environment.keys.include?("DEMO_X_TOKEN")
ctr = Demo::Counter.new
3.times { ctr.inc }
p ctr.value
p Demo::Lenient.three(3), Demo::Lenient.pairs(4), Demo::Lenient.mixed(true)
p Demo::Lenient.raised(ArgumentError, "boom"), Demo::Lenient.lines("one\ntwo")
