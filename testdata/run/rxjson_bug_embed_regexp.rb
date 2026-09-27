# rbs_inline: enabled

inner = /ab/i
outer = /x#{inner}y/
puts outer.inspect, outer.match?("xABy"), outer.match?("xaby"), outer.match?("xy")
dotall = /a.b/m
puts(/#{dotall}/.match?("a\nb"))
