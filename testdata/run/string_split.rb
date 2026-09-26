# rbs_inline: enabled

# chars and each_char walk characters, not bytes.
puts "héy😀".chars.inspect, "".chars.inspect, "ab".chars.size.inspect
out = [] #: Array[String]
"añb".each_char { |c| out << c.upcase }
puts out.inspect

# each_char is an iterator: break and next are plain loop control.
n = 0
"stop here".each_char do |c|
  break if c == " "
  next if c == "o"

  n += 1
end
puts n

#: (String) -> Integer
def vowels(s)
  s.each_char { |c| return 1 if "aeiou".include?(c) }
  0
end
puts vowels("xyz"), vowels("xa")

# lines keeps the separator and never yields a trailing empty line.
puts "a\nb\n".lines.inspect, "a\nb".lines.inspect, "".lines.inspect, "\n\n".lines.inspect
puts "a\r\nb".lines.inspect, "no newline".lines.inspect, "x\n\ny".lines.size

# split with no separator (or nil) splits on runs of ASCII whitespace.
puts "one two  three".split.inspect, "  lead and trail  ".split.inspect, "tabs\tand\nnewlines".split.inspect
puts "".split.inspect, "   ".split.inspect, "a b".split(nil).inspect, "solo".split.inspect

# split with a string drops trailing empties but keeps leading and inner ones.
puts "a,b,,c,,".split(",").inspect, ",a".split(",").inspect, "".split(",").inspect, ",,,".split(",").inspect
puts "a--b--".split("--").inspect, "abc".split("x").inspect, "a,b".split(",").size

# split("") yields characters.
puts "abc".split("").inspect, "".split("").inspect, "héllo".split("").inspect

# Round trips through Array methods.
puts "1,2,3".split(",").map(&:to_i).inspect
puts "a-b-c".split("-").reverse.join("+"), "abc".chars.reverse.join.inspect
puts "the quick brown".split.map(&:capitalize).join(" ")
puts "k=v".split("=")[1].inspect, "k=".split("=")[1].inspect
