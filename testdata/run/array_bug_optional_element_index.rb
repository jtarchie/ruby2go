# skip: Array[Integer?]#[] returns a pointer to the slot (E? with E = *Integer, i.e. **Integer), so a nil element is non-nil: nil? is false, `if (w = a[i])` is taken, and inspect panics

# rbs_inline: enabled
opt = [1, nil, 3] #: Array[Integer?]
puts opt[1].nil?, opt[0].nil?, opt[9].nil?
if (w = opt[1])
  puts "truthy #{w.inspect}"
else
  puts "falsy"
end
puts opt[1].inspect
