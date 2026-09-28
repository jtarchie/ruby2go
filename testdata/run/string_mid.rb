# rbs_inline: enabled

# default inspect stays distinct from a user to_s
class Pt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: () -> String
  def to_s = "Pt(#{x})"
end

puts (Pt.new(3).inspect == Pt.new(3).to_s).inspect
puts Pt.new(3).inspect.include?("@x=3").inspect

# a module's #{self} and bare to_s/inspect dispatch to the includer
module Tagged
  #: () -> String
  def tag = "[#{self}]"

  #: () -> String
  def loud = to_s.upcase

  #: () -> String
  def shown = "<#{inspect}>"
end

class Item
  include Tagged

  #: () -> String
  def to_s = "item"

  #: () -> String
  def inspect = "#<Item>"
end

puts Item.new.tag, Item.new.loud, Item.new.shown

puts "ab".ljust(7, "xy"), "ab".rjust(6, "-="), "ab".center(9, "*+"), "é".ljust(3, "ü")
puts "hello world".delete("lo"), "hello".delete("a-k"), "hello".delete("^l")
begin
  "x".ljust(3, "")
rescue ArgumentError => e
  puts e.message
end

puts format("%05.2f|%-4s|%x|%X|%o|%b|%#x|%#b|%+d|% d|%e|%E|%g|%g|%g|%c|%c|%%|%p|%s", 3.14159, "ab", 255, 255, 8, 5, 255, 5, 3, 3, 12345.678, 0.00012, 1234567.0, 100000.0, 0.0001, 65, "hey", nil, :sym)
puts format("%-10s|%10s|%.2s|%5.1f%%", "hi", "there", "abc", 99.5)
puts format("%<a>s and %<b>05.1f", a: "x", b: 2.25), format("%{a}-%{b}", a: 1, b: nil)
puts format("%*d|%-*d|", 5, 1, 4, 2), format("%d", 3.99), format("%d", -3.5), format("%f", 1), format("%.0f %.0f %.0f", 0.5, 1.5, 2.5)
puts format("%s", [1, "a"]), format("%3d|%-3d|%03d", -5, -5, -5), format("%.3g|%10.4f|%-10.2e|", 3.14159, Math::PI, 1234.5)
puts format("%f %f %e", Float::INFINITY, -Float::INFINITY, Float::NAN), format("%B %d", 10, "0x1f")
puts sprintf("%08.3f", -3.14159), "%s and %p" % ["x", nil], "%05d" % 42, "%.1f%%" % 12.34, "%-5s|" % :ab
printf("%d-%s\n", 1, "two")
begin
  format("%d %d", 1)
rescue ArgumentError => e
  puts e.message
end
begin
  format("%d", "abc")
rescue ArgumentError => e
  puts e.message
end
begin
  format("%<x>s", y: 1)
rescue KeyError => e
  puts e.message
end

p "0b101".to_i(0), "ff".to_i(16), "z".to_i(36), "12abc".to_i(10), "ff".hex, "-0x1A".hex, "777".oct, "0b11".oct, "junk".hex
p "az".succ, "zz".succ, "a9".succ, "Zz".succ, "1.9".next, "".succ
p "hello world".count("lo"), "hello".count("a-y"), "hello".count("^l"), "aaabbbccc".squeeze, "aaabbbccc".squeeze("a"), "mississippi".squeeze("sp")
p "Hello World".swapcase, "abc".casecmp("ABD"), "abc".casecmp?("ABC"), "abc".casecmp?("abd")
p "hello".start_with?("x", "he"), "hello".end_with?("x", "lo"), "hello".delete_prefix("he"), "hello".delete_suffix("lo"), "hello".delete_prefix("x")
p "key=value=x".partition("="), "key=value=x".rpartition("="), "abc".partition("z"), "abc".rpartition("z")
p "abc".chop, "a\r\n".chop, "abc".chr, "".chr, "héllo".ascii_only?, "hello".ascii_only?
p "a,b,c,d".split(",", 2), "a b  c".split(" ", 2), "a,b,,".split(",", -1), "abc".each_char.to_a, "a\nb\n".each_line.to_a
"x\ny".each_line { |l| p l }
p "hello"[1..], "hello".slice(1, 3), "ab" * 3
p "Straße".upcase, "ÀÉÎ".downcase, "hello world".capitalize, "HELLO".capitalize
p "%05.1f" % 3.14159, "abc".tr("a-c", "A-C"), "abc".bytes.sum

