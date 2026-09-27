# rbs_inline: enabled

class Foo
  #: () -> String
  def inspect = "#<Foo>"
end

inferred = { "a" => nil, "b" => 2 }
puts inferred.inspect
opt = { "a" => nil, "b" => 2 } #: Hash[String, Integer?]
puts opt.inspect, opt.to_s
nil_key = { nil => 1, "a" => 2 } #: Hash[String?, Integer]
puts nil_key.inspect
# Non-nil values of a nilable struct-class or Array type fail too.
objs = { "f" => Foo.new } #: Hash[String, Foo?]
puts objs.inspect
lists = { "l" => [1] } #: Hash[String, Array[Integer]?]
puts lists.inspect
