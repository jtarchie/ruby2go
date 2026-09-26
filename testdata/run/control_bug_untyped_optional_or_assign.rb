# skip: `v ||= x` on an `untyped?` local (from Hash[String, untyped]#[]) emits `!v != nil` and assigns a String to a *any, so go build fails

# rbs_inline: enabled

h = { "f" => false, "s" => "set" } #: Hash[String, untyped]
%w[f s missing].each do |k|
  v = h[k]
  v ||= "filled"
  puts v.inspect
end
