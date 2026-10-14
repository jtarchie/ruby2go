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
p Demo::Lenient.uri_path("http://h/a/b"), Demo::Lenient.uri_path("x")
p Demo::Lenient.merged({ x: 1 })
fs = DemoFormulaStruct.new(desc: "d", bottle_present: true)
p fs.desc, fs.bottle?, fs.head?
fh = DemoFormulaStruct.from_hash({ "desc" => "h", "head_present" => true, "junk" => 1 })
p fh.desc, fh.head?, fh.bottle?
p DemoSystemCommand.new("a").input, DemoSystemCommand.new(%w[b c]).input
p DemoVersion.new(1) < DemoVersion.new(2), [DemoVersion.new(3), DemoVersion.new(1)].max.v, DemoVersion.new(1) <=> "x"
bes = [DemoBundle::Entry.new("a"), DemoBundle::Entry.new("b")]
p DemoBundle::Ext.run(bes), DemoBundle::Vscode.run(bes), DemoBundle::Vscode.cleanup_items(bes)
p DemoBottle.new("wget").name, DemoBottle.new(nil, name: "curl").name
begin
  DemoBottle.strict(nil)
rescue TypeError => e
  p e.class
end
lg = Demo::Lenient.logger
p lg.call(1, "x", nil), lg.call(3, "p", "m")
p Demo::Lenient.level_of, Demo::Lenient.level_of(:SEV_DEBUG)
p Demo::Lenient.sent("aBc")
p Demo::Lenient.missing(true), Demo::Lenient.missing(false)
tagb = [{ "a" => :x }, { "b" => %i[y z] }, { "c" => nil }] #: Array[Hash[String, untyped]]
p tagb.map { |h| Demo::Lenient.tags_of(h) }
p DemoPool.new(name: "q", size: 4) { |x| x * 2 }.report, DemoPool.new.report
p DemoPlainPool.new(size: 3).size
p DemoTimerService.new.schedule(4)
dcp = DemoCachedPool.new
dcp.ns_initialize({ size: 5 })
p dcp.size
p DemoThen.new([3]) { |x| x * 10 }.run, DemoExec.new.post(1, 2) { |a, b| a + b }
ENV["DEMO_TOKEN"] = "s"
p ENV.demo_clear_sensitive! { ENV["DEMO_TOKEN"] }, ENV["DEMO_TOKEN"], ENV.demo_sensitive?("a_password")
p [DemoPostExec.new, DemoTimerSet.new].map { |x| x.run_all([1, 2]) }
p DemoTimerSet.new.post(7, 1) { |v| v * 3 }
dev = DemoEvent.new
dev.set
dev.set
p dev.count
dfe = DemoFallbackExec.new
dfe.go
p dfe.log
div = DemoIVar.new
div.add_observer { |x| x * 2 }
div.with_observer { |x| x + 1 }
p div.demo_notify(10)
lines = [] #: Array[untyped]
DemoLineSource.new.each_line_from(["a", "b"]) { |t, l| lines << [t, l] }
p lines
nlog = [] #: Array[String]
p Demo::Lenient.first_over([1, 5, 20, 30], 10, nlog), Demo::Lenient.first_over([1], 10, nlog), nlog
dlog = [] #: Array[String]
p Demo::Lenient.depth_find(3, dlog), dlog, Demo::Lenient.void_early(dlog), dlog
dd = DemoDelayish.new
p dd.dupv, dd.value_with(:v)
dv = DemoVisitor.new
p dv.collect_yields, dv.as_proc("ab").call, dv.pair
