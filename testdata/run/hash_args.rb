# rbs_inline: enabled

# Decision 23: `f(a: 1)` on a method without keyword parameters passes a Hash.

#: (Hash[Symbol, Integer]) -> String
def show(opts) = opts.inspect

#: (String, ?Hash[Symbol, String]) -> String
def tag(name, attrs = {})
  rendered = attrs.map { |key, value| " #{key}=\"#{value}\"" }.join
  "<#{name}#{rendered}>"
end

#: (Hash[String, Integer]) -> Integer
def total(counts) = counts.values.reduce(0) { |sum, n| sum + n }

#: (Hash[Symbol, untyped]) -> String
def loose(o) = o.inspect

#: (Integer, Hash[Symbol, Integer]) -> Integer
def scaled(n, opts) = n * opts.fetch(:by, 1)

#: (untyped) -> String
def any_arg(x) = x.inspect

#: (*untyped) -> String
def rest(*args) = args.inspect

#: (Integer, *untyped) -> String
def rest2(n, *args) = "#{n} #{args.inspect}"

#: (?Hash[Symbol, Integer]?) -> String
def opt_nil(o = nil) = o ? o.inspect : "none"

#: (Hash[untyped, Integer]) -> String
def mixed(o) = o.inspect

#: (Hash[Symbol, Integer]) { (Integer) -> Integer } -> Integer
def with_blk(o) = yield(o.fetch(:n, 0))

# Ruby fills optional positionals left to right, so a braceless hash lands in the first.
#: (Integer, ?Hash[Symbol, Integer], ?Hash[Symbol, Integer]) -> String
def two_opts(n, a = {}, b = {}) = "#{n} #{a.inspect} #{b.inspect}"

#: () { (Hash[Symbol, Integer]) -> void } -> void
def give = yield(a: 1)

#: () { (Hash[Symbol, Integer]) -> String } -> String
def give_v = yield(b: 2, c: 3)

class Base
  #: (Hash[Symbol, Integer]) -> void
  def initialize(opts)
    @opts = opts
  end

  #: () -> String
  def to_s = @opts.inspect
end

class Sub < Base
  #: () -> void
  def initialize
    super(a: 1, b: 2)
  end
end

class Box
  #: (Hash[Symbol, Integer]) -> String
  def show(o) = o.inspect

  #: () -> String
  def call_self = show(k: 1)
end

class Conn
  attr_reader :opts #: Hash[Symbol, untyped]

  #: (String, ?Hash[Symbol, untyped]) -> void
  def initialize(host, opts = {})
    @host = host
    @opts = opts
  end

  #: (Hash[Symbol, untyped]) -> Conn
  def with(extra) = Conn.new(@host, @opts.merge(extra))

  #: (Hash[String, Integer]) -> Integer
  def self.weigh(w) = w.values.reduce(0) { |a, b| a + b }

  #: () -> String
  def to_s = "#{@host} #{@opts.inspect}"
end

puts show(a: 1, b: 2)
puts show a: 3
puts show({ c: 4 })
puts show({})
puts tag("br"), tag("a", href: "/x", id: "y"), tag("p", {}), tag("i", "é": "ü")
puts total("a" => 1, "b" => 2), total("z" => -5), total({})
puts loose(port: 8080, host: "h", on: true, none: nil, list: [1, 2])
puts scaled(3, by: 4), scaled(3, {}), scaled(-2, by: 0)

c = Conn.new("h", port: 1, tls: true)
puts c
puts Conn.new("bare")
puts c.with(port: 2, retry: 3)
puts c.with({})
puts c
puts Conn.weigh("x" => 1, "y" => 2)
puts Conn.weigh "z" => 3
puts c.opts[:port].inspect, c.opts[:missing].inspect

# Core methods take braceless hashes too.
puts(a: 1)
puts 1, b: 2
puts [a: 1].inspect, [1, b: 2].inspect
k = { a: 1 } #: Hash[Symbol, Integer]
puts k.merge(b: 2).inspect, k.merge(a: 9).inspect, k.inspect

# Braceless hashes into untyped, rest, nilable, mixed-key and block-taking methods.
puts any_arg(a: 1), any_arg("x" => [1])
puts rest(a: 1), rest(1, a: 2), rest, rest2(1, b: 2)
puts opt_nil(z: 1), opt_nil, opt_nil(nil)
puts mixed("a" => 1, b: 2)
puts with_blk(n: 5) { |x| x * 2 }
puts(with_blk(n: 6) do |x| x + 1 end)
puts two_opts(1), two_opts(1, x: 1), two_opts(1, { x: 1 }, y: 2)
give { |gh| puts gh.inspect }
puts give_v { |gh| gh.keys.inspect }
# ... into super, and into a call on self.
puts Sub.new, Box.new.call_self
shown = Box.new.show k2: 7
puts shown
