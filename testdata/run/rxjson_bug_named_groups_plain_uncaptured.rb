# skip: with named groups present, MRI does not capture plain (...) groups; rb2go numbers them (/(?<a>x)(y)/ has captures.size 2, MRI 1)

# rbs_inline: enabled

m = /(?<a>x)(y)/.match("xy")
if m
  puts m.captures.size, m[1].inspect, m[2].inspect, m.captures.join(",")
end
