# skip: an `untyped?` value (Hash[String, untyped]#[], Array[untyped]#[]) holding false or nil is truthy in if/?:/&& and not nil? (pointer compared with nil); decisions 20/33 say untyped? is untyped and conditions use truthiness

# rbs_inline: enabled

h = { "t" => true, "f" => false, "n" => nil, "z" => 0 } #: Hash[String, untyped]
%w[t f n z missing].each do |k|
  puts "#{k}: #{h[k] ? "truthy" : "falsy"}"
  puts "  if" if h[k]
  v = h[k]
  puts "  local if" if v
  puts "  nil? #{h[k].nil?}"
  puts "  && #{(h[k] && "rhs").inspect}"
end
arr = [false, nil, 1] #: Array[untyped]
puts "arr[0] truthy" if arr[0]
puts "arr[1] truthy" if arr[1]
puts "arr[2] truthy" if arr[2]
