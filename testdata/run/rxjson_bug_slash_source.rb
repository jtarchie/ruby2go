# rbs_inline: enabled

x = "b/c"
puts(/a\/b/.source)
puts %r{a/b}.inspect, %r{a/b}.to_s, %r{a/b}.source
puts(/a#{x}/.inspect, /a#{x}/.to_s, /a#{x}/.source)
w = "w"
puts %r{#{w}/x}.inspect, /#{w}\//.source, /#{w}\//.inspect
