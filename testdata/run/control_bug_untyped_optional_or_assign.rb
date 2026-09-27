# rbs_inline: enabled

h = { "f" => false, "s" => "set" } #: Hash[String, untyped]
%w[f s missing].each do |k|
  v = h[k]
  v ||= "filled"
  puts v.inspect
end
