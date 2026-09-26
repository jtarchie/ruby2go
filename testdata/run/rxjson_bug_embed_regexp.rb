# skip: interpolating a Regexp embeds its to_s, (?i-mx:...), which RE2 rejects with RegexpError (MRI composes the patterns)

# rbs_inline: enabled

inner = /ab/i
outer = /x#{inner}y/
puts outer.inspect, outer.match?("xABy"), outer.match?("xaby"), outer.match?("xy")
dotall = /a.b/m
puts(/#{dotall}/.match?("a\nb"))
