# rbs_inline: enabled

# String building: interpolation, format/%, ljust/rjust, to_s(base), Integer/Float to_s, upcase, center, StringIO.
require "stringio"

out = StringIO.new
100_000.times do |i|
  out << format("%05d|%-8s|%8.3f|%x\n", i, "n#{i % 97}", i / 7.0, i)
end
s = out.string
puts s.bytesize, s.lines[12_345]

rows = Array.new(50_000) { |i| i.to_s(2).rjust(20, "0") + " " + (i * 1.5).to_s.ljust(10) + i.to_s.center(9, "*") }
buf = rows.join("\n")
puts buf.size, buf.lines.last

words = %w[alpha beta gamma delta epsilon]
acc = 0
200_000.times do |i|
  w = words[i % 5] || ""
  acc += "#{w.upcase}-#{w.capitalize}-#{w.reverse}:#{i}".length
end
puts acc

puts (1..100_000).map { |i| "%.2f" % (i / 3.0) }.join(",").length
puts (1..100_000).sum { |i| i.to_s.sum + i.digits.sum }
