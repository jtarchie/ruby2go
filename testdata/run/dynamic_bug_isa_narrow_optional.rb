# rbs_inline: enabled

#: (Integer) -> String?
def word(n) = n.positive? ? "w" * n : nil

#: (String) -> Integer
def len(s) = s.size

s = word(2)
puts len(s) if s.is_a?(String)
t = word(0)
puts (t.is_a?(String) ? len(t) : -1).inspect
