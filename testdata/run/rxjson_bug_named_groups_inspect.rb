# rbs_inline: enabled

m = /(?<year>\d+)-(?<mon>\d+)(?<day>-\d+)?/.match("2024-05")
if m
  puts m[1].inspect, m[2].inspect, m[3].inspect, m.captures.size
  puts m.inspect
end
