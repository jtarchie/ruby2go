# Standalone on purpose: the views' fallbacks call the + and === dispatchers, which no call in this program notes.
nums = [1, 2] #: untyped
puts nums.sum
words = %w[a b] #: untyped
puts words.sum("")
mixed = [1, "a", 2.5] #: untyped
p mixed.grep(Integer)
p mixed.grep_v(String)
