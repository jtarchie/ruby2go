# rbs_inline: enabled

# adjacent literals and a line continuation concatenate
n_interp = 1
puts "t" "u" "#{n_interp}" 'v'
puts "total: #{n_interp} " \
  "items"

# clamp with min > max raises
begin
  puts "b".clamp("c", "a")
rescue ArgumentError => e
  puts "clamp: #{e.message}"
end

# Kernel#to_s/inspect show a heap address
class Foo
end

foo_obj = Foo.new
puts foo_obj.to_s.start_with?("#<Foo:0x").inspect, foo_obj.to_s.end_with?(">").inspect, "#{foo_obj}".include?(":0x").inspect
puts foo_obj.inspect.start_with?("#<Foo:0x").inspect

# a literal passed to a generic param keeps String
class Fmt
  # @rbs [T] (T) -> String
  def show(x) = "<#{x}>|#{x.inspect}"
end

fmt = Fmt.new
puts fmt.show("s"), fmt.show(12), fmt.show(2.5), fmt.show(:sym), fmt.show("a" + "b")
puts fmt.show(nil)

# invalid UTF-8 bytes inspect as \xNN
puts "\xff".inspect
puts "a\xe1b".inspect

# String#* with a negative count raises
begin
  puts "ab" * -1
rescue ArgumentError => e
  puts "mul: #{e.message}"
end

# != compares by value, not identity
ch = "abc".chars[1]
puts (ch != "b").inspect
built = "a" + "b"
puts (built != "ab").inspect, ("x".upcase != "X").inspect, ("ab".dup != "ab").inspect
ne_count = 0
"abc".each_char { |c| ne_count += 1 if c != "b" }
puts ne_count
puts ("b" != "abc"[1]).inspect

# a String/Symbol passed as Object keeps its class

#: (Object) -> String
def kind(o) = o.class.name

puts kind("a"), kind(:a)

# Object-typed values interpolate, inspect and is_a? as themselves

#: (Object) -> String
def desc(o) = "#{o}|#{o.inspect}|#{o.is_a?(String)}|#{o.is_a?(Symbol)}"

puts desc("a"), desc(:a), desc("a" + "b"), desc(1)
objs = ["s", :t, 2.5] #: Array[Object]
puts objs.map { |o| o.to_s }.inspect
puts objs.inspect

# ord on an empty string raises
begin
  puts "".ord
rescue ArgumentError => e
  puts "ord: #{e.message}"
end

# puts flattens arrays and prints nil elements as blank lines
b_opt = ["x", nil] #: Array[String?]
puts b_opt
puts "after"
u_nested = [1, [2, nil]] #: untyped
puts u_nested
puts "end"

# puts [] prints a blank line, nested empties print nothing
puts "a"
puts []
puts "b"
puts [[], []]
puts "c"

# puts flattens nested arrays and tuples
puts [1, [2, [3]]]
tup = [1, "a"]
puts tup

# puts and print return nil
puts puts("a").inspect
puts print("b\n").nil?

# split(" ") is awk-style whitespace splitting
puts "a  b ".split(" ").inspect
puts " a b".split(" ").inspect

# awk-style split only breaks on ASCII whitespace
puts "a\u00a0b\u3000c d".split.inspect
puts "x\u00a0y".split(nil).size

# sub/gsub replacement strings honor backreferences
puts "abc".sub("b", "\\0\\0").inspect
puts "a'b".gsub("'", "\\'").inspect
puts "x-y".gsub("-", "<\\&>").inspect
puts "abc".gsub("b", "\\`").inspect
puts "abc".sub("b", "[\\1]").inspect
puts "a".sub("a", "\\\\").inspect

# to_f parses the longest numeric prefix
puts "3.5abc".to_f
puts "1.5 kg".to_f
puts "1e".to_f, "1.5.3".to_f, "1__0".to_f, "  +1.5e2x".to_f
puts "Infinity".to_f
puts "NaN".to_f
puts "inf".to_f, "0x1p3".to_f

# to_i accepts underscores between digits
puts "1_000".to_i
puts "12_345xyz".to_i

# tr with a repeated from-char uses its first mapping
puts "hello".tr("ll", "xy").inspect

# tr with an empty to-set deletes
puts "hello".tr("l", "").inspect

# tr ranges and ^ negation
puts "hello".tr("a-y", "b-z").inspect
puts "hello".tr("^l", "*").inspect

# strip/to_i/to_f do not treat Unicode spaces as whitespace
puts "\u00a0x\u00a0".strip.inspect
puts "\u3000x".strip.inspect
puts "\u00a012".to_i, "\u300012".to_i
puts "\u00a01.5".to_f

# .class on untyped String/Symbol

#: (untyped) -> untyped
def ident_class(v) = v

puts ident_class("a").class, ident_class(:a).class

# <=> on untyped String/Symbol

#: (untyped) -> untyped
def ident_cmp(v) = v

s_cmp = ident_cmp("b")
puts (s_cmp <=> "a").inspect, (s_cmp <=> "b").inspect
y_cmp = ident_cmp(:b)
puts (y_cmp <=> :c).inspect

# case mapping follows Unicode special casing
puts "straße".upcase.inspect
puts "ß".capitalize.inspect
puts "İ".downcase.inspect
puts "ﬀ".upcase.inspect
puts "ǆa".capitalize.inspect

# Non-printable non-ASCII is \uXXXX (\u{X} above U+FFFF); format (Cf) and private-use characters print raw, like MRI.
puts "\u0085| | | |­|​|".inspect
puts "\u{10FFFF}|\u{E0001}|\u{1F600}|�|é".inspect
# Invalid bytes are \xNN, one per byte; a surrogate encoding is invalid too.
puts "\xed\xa0\x80|\xe1\x80|\xc3".inspect
# Named escapes and the #{ #$ #@ guard still apply around them.
puts "\e\a\b\f\v\u0085\#{x}\#$y\#@z#a".inspect
puts [" ", "\xff"].inspect
# ASCII-only symbols are US-ASCII in MRI, so controls stay \xNN.
puts :"a\x00b".inspect
puts({"a\x7f": 1}.inspect)
