# Crystal port of benchmarks/regexp.rb, for the README comparison column.
# Regexes run on PCRE2 (Ruby: Onigmo). StringScanner has no getch; scan(1) stands in (never reached on this input).
# Scanner matches pass NO_UTF_CHECK (the documented idiom for an already-valid string): without it PCRE2
# re-validates the whole 128 KB source on every token, ~9 s instead of ~0.1 s.
require "string_scanner"

LINE = "2026-10-20 12:34:56 GET /api/v1/users/42?x=1 200 1532ms user=alice@example.com"
lines = Array.new(20_000) { |i| LINE.sub("42", i.to_s).sub("1532", (i % 2000).to_s) }

status = Hash(String, Int32).new(0)
ms = 0
lines.each do |l|
  if m = l.match(/ (GET|POST) (\S+) (\d{3}) (\d+)ms/)
    status[m[3]? || ""] += 1
    ms += (m[4]? || "0").to_i
  end
end
puts status.inspect, ms

emails = lines.sum { |l| l.scan(/\w+@\w+\.com/).size }
puts emails

masked = lines.map { |l| l.gsub(/\d/) { |d| ((d.to_i + 1) % 10).to_s } }
puts masked.last
puts lines.map { |l| l.gsub(/[aeiou]/) { |v| "43_10"[("aeiou".index(v) || 0)]?.to_s } }.first
puts lines.sum { |l| l.split(/[\s\/?=]+/).size }
puts lines.count { |l| l =~ /users\/1\d*\?/ }
puts lines.map { |l| l.tr("a-z", "b-za").delete("0-9").squeeze }.uniq.size

tokens = 0
sums = 0
src = "let x = 10 + foo(20, 30) * bar; " * 4000
ss = StringScanner.new(src)
valid = Regex::MatchOptions::NO_UTF_CHECK
until ss.eos?
  if ss.skip(/\s+/, options: valid)
    next
  elsif n = ss.scan(/\d+/, options: valid)
    sums += n.to_i
  elsif ss.scan(/[a-z_]\w*/, options: valid) || ss.scan(/[(),;=+*]/, options: valid)
    tokens += 1
  else
    ss.scan(1)
  end
end
puts tokens, sums
