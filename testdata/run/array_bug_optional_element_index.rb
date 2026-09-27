# rbs_inline: enabled
opt = [1, nil, 3] #: Array[Integer?]
puts opt[1].nil?, opt[0].nil?, opt[9].nil?
if (w = opt[1])
  puts "truthy #{w.inspect}"
else
  puts "falsy"
end
puts opt[1].inspect
