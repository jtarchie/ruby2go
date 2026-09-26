# rbs_inline: enabled

class Integer
  #: () -> Integer
  def double = self * 2

  #: (Integer) -> bool
  def divisible_by?(n) = (self % n).zero?

  #: () -> Integer
  def factorial
    acc = 1
    1.upto(self) { |i| acc *= i }
    acc
  end

  #: () -> String
  def ordinal
    return "#{self}th" if (self % 100).between?(11, 13)
    case self % 10
    when 1 then "#{self}st"
    when 2 then "#{self}nd"
    when 3 then "#{self}rd"
    else "#{self}th"
    end
  end
end

class Float
  #: () -> bool
  def positive? = self > 0.0

  #: () -> Float
  def half = self / 2.0
end

puts 4.double.inspect, -3.double.inspect, 12.divisible_by?(4).inspect, 12.divisible_by?(5).inspect
puts 0.factorial.inspect, 5.factorial.inspect, 20.factorial.inspect
puts [1, 2, 3, 4, 11, 12, 13, 21, 22, 101, 111, 0].map { |i| i.ordinal }.inspect
puts 4.5.positive?.inspect, -0.5.positive?.inspect, 3.0.half.inspect
