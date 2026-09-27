# rbs_inline: enabled

#: (bool) -> bool?
def maybe(b) = b ? false : nil

c = maybe(true)
puts (c || "fallback").inspect
puts(c ? "truthy" : "falsy")
puts "not printed" if c
d = maybe(true)
d ||= true
puts d.inspect
e = maybe(true)
puts (e && "rhs").inspect
h = { "off" => false } #: Hash[String, bool]
puts (h["off"] || true).inspect
puts(h["off"] ? "on" : "off")
puts(h["missing"] ? "on" : "missing")
