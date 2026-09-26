# rbs_inline: enabled

# Case mapping on ASCII and on letters with a single-rune upper/lower form.
puts "Hello, World".upcase.inspect, "Hello, World".downcase.inspect, "".upcase.inspect
puts "àéîõü".upcase.inspect, "ÀÉÎÕÜ".downcase.inspect, "ÿ".upcase.inspect, "Σ".downcase.inspect
puts "123 !?".upcase.inspect, "MiXeD 42".downcase.inspect

# capitalize lowercases the rest; a non-letter first char stays as is.
puts "hELLO wORLD".capitalize.inspect, "".capitalize.inspect, "123abc".capitalize.inspect
puts " x".capitalize.inspect, "éCOLE".capitalize.inspect, "a".capitalize.inspect, "ABC".capitalize.inspect

# reverse works on characters, so multibyte text survives.
puts "héllo".reverse.inspect, "".reverse.inspect, "a😀b".reverse.inspect, "ab".reverse.reverse.inspect

# strip family: ASCII whitespace only in these inputs.
puts "  padded  ".strip.inspect, " \t\n\v\f\rx \t\n\v\f\r".strip.inspect, "".strip.inspect, "   ".strip.inspect
puts " \t\n\v\f\rx \t\n\v\f\r".lstrip.inspect, " \t\n\v\f\rx \t\n\v\f\r".rstrip.inspect
puts "  in side  ".lstrip.inspect, "  in side  ".rstrip.inspect, "x".lstrip.inspect

# chomp removes exactly one trailing \n, \r\n or \r.
puts "a\n".chomp.inspect, "a\r\n".chomp.inspect, "a\r".chomp.inspect, "a\n\n".chomp.inspect
puts "a\n\r".chomp.inspect, "a\r\r\n".chomp.inspect, "a".chomp.inspect, "".chomp.inspect, "\n".chomp.inspect

# Justification pads by character count; odd padding goes to the right in center.
puts "x".center(5).inspect, "ab".center(5).inspect, "abc".center(6).inspect, "héllo".center(9).inspect
puts "abc".center(2).inspect, "abc".center(-1).inspect, "".center(3).inspect, "abc".center(3).inspect
puts "x".ljust(3).inspect, "é".ljust(3).inspect, "abc".ljust(0).inspect, "abc".ljust(-2).inspect
puts "x".rjust(3).inspect, "é".rjust(3).inspect, "abc".rjust(-5).inspect, "".rjust(2).inspect
puts "|" + "id".ljust(4) + "|" + "7".rjust(3) + "|"

# sub replaces the first match of a literal pattern, gsub all of them.
puts "hello".sub("l", "L").inspect, "hello".gsub("l", "L").inspect, "aaa".sub("a", "b").inspect
puts "a.b.c".gsub(".", "-").inspect, "abc".gsub("x", "y").inspect, "abc".gsub("abc", "").inspect
puts "abc".gsub("", "-").inspect, "abc".sub("", "-").inspect, "".gsub("", "x").inspect
puts "héllo wörld".gsub("ö", "o").inspect, "aaaa".gsub("aa", "b").inspect, "a*b".sub("*", "+").inspect

# tr maps characters one to one; a short to-list repeats its last character.
puts "hello".tr("el", "ip").inspect, "hello".tr("elo", "x").inspect, "héllo".tr("é", "e").inspect
puts "hello".tr("xyz", "abc").inspect, "".tr("a", "b").inspect, "abcabc".tr("abc", "cab").inspect

# Chained transforms keep each intermediate result immutable.
s = "  Mixed Case  "
t = s.strip.downcase.tr(" ", "_")
puts t.inspect, s.inspect
puts "ab".upcase * 2 + "cd".center(4).upcase
