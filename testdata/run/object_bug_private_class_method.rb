# skip: `def self.x` after `private` becomes private in rb2go ("private method hidden called on singleton(Counter)"); MRI's `private` does not affect singleton defs

# rbs_inline: enabled

class Counter
  #: () -> String
  def self.shown = "shown"

  private

  #: () -> String
  def self.hidden = "still public"

  #: () -> String
  def inst = "private instance"
end

puts Counter.shown, Counter.hidden
