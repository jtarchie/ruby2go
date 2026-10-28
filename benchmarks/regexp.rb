# rbs_inline: enabled

# Regexp matching, captures, gsub with a hash and a block, scan, split on a pattern, and StringScanner.
require "strscan"

LINE = "2026-10-20 12:34:56 GET /api/v1/users/42?x=1 200 1532ms user=alice@example.com".freeze
lines = Array.new(20_000) { |i| LINE.sub("42", i.to_s).sub("1532", (i % 2000).to_s) }

status = Hash.new(0) #: Hash[String, Integer]
ms = 0
lines.each do |l|
  if (m = l.match(/ (GET|POST) (\S+) (\d{3}) (\d+)ms/))
    status[m[3] || ""] += 1
    ms += (m[4] || "0").to_i
  end
end
puts status.inspect, ms

emails = lines.sum { |l| l.scan(/\w+@\w+\.com/).size }
puts emails

masked = lines.map { |l| l.gsub(/\d/) { |d| ((d.to_i + 1) % 10).to_s } }
puts masked.last
puts lines.map { |l| l.gsub(/[aeiou]/) { |v| "43_10"["aeiou".index(v) || 0] || "" } }.first
puts(lines.sum { |l| l.split(%r{[\s/?=]+}).size })
puts(lines.count { |l| l =~ %r{users/1\d*\?} })
puts lines.map { |l| l.tr("a-z", "b-za").delete("0-9").squeeze }.uniq.size

tokens = 0
sums = 0
src = "let x = 10 + foo(20, 30) * bar; " * 4000
ss = StringScanner.new(src)
until ss.eos?
  if ss.skip(/\s+/)
    next
  elsif (n = ss.scan(/\d+/))
    sums += n.to_i
  elsif ss.scan(/[a-z_]\w*/) || ss.scan(/[(),;=+*]/)
    tokens += 1
  else
    ss.getch
  end
end
puts tokens, sums
