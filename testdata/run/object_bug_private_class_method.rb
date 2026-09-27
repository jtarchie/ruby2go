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
