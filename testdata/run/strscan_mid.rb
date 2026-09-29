# rbs_inline: enabled

require "strscan"

s = StringScanner.new("2024-09-28")
s.scan(/(?<year>\d+)-(?<month>\d+)-(?<day>\d+)/)
puts s.named_captures.inspect
puts s.values_at(0, 1, 2, 3).inspect
puts s.values_at(-1, -2).inspect

s2 = StringScanner.new("no match here")
s2.scan(/xyz/)
puts s2.named_captures.inspect

s3 = StringScanner.new("abc")
s3 << "def"
puts s3.concat("ghi").string
puts s3.rest

s4 = StringScanner.new("foo5bar")
puts s4.scan_full(/\w+/, true, true).inspect
puts s4.pos

s5 = StringScanner.new("test string")
puts s5.scan_full(/\w+/, false, true).inspect
puts s5.pos
puts s5.scan_full(/\w+/, false, false).inspect
puts s5.pos
puts s5.scan_full(/xyz/, true, true).inspect

s6 = StringScanner.new("hello world")
puts s6.search_full(/wor/, true, true).inspect
puts s6.pos

s7 = StringScanner.new("hello world")
puts s7.search_full(/wor/, false, true).inspect
puts s7.pos
puts s7.search_full(/wor/, false, false).inspect
puts s7.search_full(/xxx/, true, true).inspect
