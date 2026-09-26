# skip: MatchData#inspect labels named groups by number (1:"2024"); MRI prints the names (year:"2024")

# rbs_inline: enabled

m = /(?<year>\d+)-(?<mon>\d+)(?<day>-\d+)?/.match("2024-05")
if m
  puts m[1].inspect, m[2].inspect, m[3].inspect, m.captures.size
  puts m.inspect
end
