# rbs_inline: enabled

require "date"

d = Date.new(2026, 9, 28)
puts d.httpdate
puts d.rfc3339
puts d.jisx0301

puts Date.httpdate(d.httpdate) == d
puts Date.rfc3339(d.rfc3339) == d
puts Date.jisx0301(d.jisx0301) == d

cases = [[1868, 9, 8], [1872, 12, 31], [1873, 1, 1], [1912, 7, 29], [1912, 7, 30], [1926, 12, 24], [1926, 12, 25], [1989, 1, 7], [1989, 1, 8], [2019, 4, 30], [2019, 5, 1], [1600, 1, 1]] #: Array[[Integer, Integer, Integer]]
cases.each do |y, m, dd|
  dt = Date.new(y, m, dd)
  puts dt.jisx0301
  puts Date.jisx0301(dt.jisx0301) == dt
end
