# skip: a negative MatchData#[] index that lands on group 0 returns the whole match; MRI returns nil

# rbs_inline: enabled

m = "abc".match(/b/)
puts m[-1].inspect if m
m2 = /(a)/.match("a")
puts m2[-1].inspect, m2[-2].inspect, m2[-3].inspect if m2
