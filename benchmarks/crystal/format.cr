# Crystal port of benchmarks/format.rb, for the README comparison column.
# StringIO is IO::Memory; Ruby's String#sum (16-bit byte checksum) is written out.

# String building: interpolation, format/%, ljust/rjust, to_s(base), Integer/Float to_s, upcase, center, StringIO.

out = IO::Memory.new
100_000.times do |i|
  out << sprintf("%05d|%-8s|%8.3f|%x\n", i, "n#{i % 97}", i / 7.0, i)
end
s = out.to_s
puts s.bytesize, s.lines(chomp: false)[12_345]

rows = Array.new(50_000) { |i| i.to_s(2).rjust(20, '0') + " " + (i * 1.5).to_s.ljust(10) + i.to_s.center(9, '*') }
buf = rows.join("\n")
puts buf.size, buf.lines.last

words = %w[alpha beta gamma delta epsilon]
acc = 0
200_000.times do |i|
  w = words[i % 5]
  acc += "#{w.upcase}-#{w.capitalize}-#{w.reverse}:#{i}".size
end
puts acc

puts (1..100_000).map { |i| "%.2f" % (i / 3.0) }.join(",").size
puts (1..100_000).sum { |i| (i.to_s.each_byte.sum(0) { |b| b.to_i } & 0xFFFF) + i.digits.sum }
