# rbs_inline: enabled

m = /(?<a>x)(y)/.match("xy")
if m
  puts m.captures.size, m[1].inspect, m[2].inspect, m.captures.join(",")
end
