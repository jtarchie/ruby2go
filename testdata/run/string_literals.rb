# rbs_inline: enabled

class Pt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end

  #: () -> String
  def to_s = "Pt(#{x})"
end

# Single quotes keep #{} and \n literally; only \' and \\ are escapes.
puts 'single #{not} \n raw'.inspect, '\'q\' \\ \n'.inspect, ''.inspect

# Double-quoted escapes, including \u{} with several codepoints and octal/hex.
puts "tab\tnl\\n".inspect, "é\u{1F600}\x41\e\s|".inspect, "\101\u{41 42}".inspect
puts "\0".size, "\cA".ord

# Percent literals with every delimiter pair, nesting included.
puts %q(paren (nested) 'q').inspect, %Q[brack #{1 + 2}].inspect, %(par (x)).inspect
puts %<ang>.inspect, %|bar|.inspect, %Q{br {n} #{1}}.inspect
puts %w[a b c].inspect, %i[a b c].inspect, %w[].inspect

# Character literals are one-character strings.
puts ?a.inspect, ?\n.inspect, ?é.inspect, ?\s.inspect, ?\t.inspect, (?a + ?b).inspect
puts %q(a\)b).inspect, "\C-a".ord, "\c?".bytesize

# Adjacent plain literals and backslash-newline continue one string.
puts "adj" "acent" 'lits'
puts "line one \
continued".inspect

# Heredocs: squiggly strips indentation, dash keeps it, quoted is raw.
n = 5
s = "str"
h = <<~EOS
  indented #{n}
    more
  done
EOS
puts h.inspect
r = <<~'RAW'
  raw #{n} \t
RAW
puts r.inspect
d = <<-DASH
    keep #{s}
    DASH
puts d.inspect
puts <<~EOS.strip.upcase
  shout #{"it"}
EOS
# Squiggly dedent skips blank lines, counts a tab, and keeps an interpolated value's own spaces.
val = "  val"
dedent = <<~EOS

  a

    b
	tab
    #{val}
EOS
puts dedent.inspect
m = <<~EOS
  x #{[1, 2].map { |i|
    i * 2
  }.join(",")} y
EOS
puts m.inspect
a, b = <<~A, <<~B
  first
A
  second
B
puts a.inspect, b.inspect

# Interpolation calls to_s on each type; nil (typed or optional) is empty.
f = 2.5
sym = :sy
nl = nil #: Integer?
some = 7 #: Integer?
t = true
arr = [1, 2] #: Array[Integer]
puts "#{n} #{f} #{s} #{sym} [#{nl}] [#{some}] #{t} #{arr} #{nil} #{false}"
puts "#{Pt.new(3)} and #{n > 3 ? "big" : "small"}"
puts "nested #{"inner #{n + 1}"}", "#{"#{"#{1}"}"}"
puts "#{s.upcase}#{s.size}", "#{[1, 2].map { |x| x * 2 }}"
puts "#{if n > 0 then "pos" else "neg" end}"
puts "#{case n when 1 then "one" when 4, 5 then "few" else "many" end}"
puts %W[a#{n} b #{s}c].inspect, %W[#{n}].size
maybe = "abc"[5]
puts "maybe=[#{maybe}] [#{"abc"[1]}]"
puts "#{-0.0} #{1e20} #{1.0e-5} #{100.0} #{1.0 / 3} #{2**62} #{-5}"
puts "sym=#{:"a b"} arr=#{[:a, "b", 1.5, nil]} h=#{{ "k" => :v }} hs=#{{ a: 1 }}"
u = 1 #: untyped
puts "u=#{u}"
puts "cls=#{String} tup=#{[1, "a"]} tri=#{[1, "a", :b]}"
puts "only #{s}", "#{s}", "#{s}#{s}", "#{n}".inspect

# Interpolation builds a fresh String each time.
parts = [] #: Array[String]
3.times { |i| parts << "p#{i}" }
puts parts.inspect
puts "a#{1}b" == "a1b"
