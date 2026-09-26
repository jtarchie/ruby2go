# rbs_inline: enabled

# Numbers and booleans held in constants, ivars, default args and multiple assignment; Kernel reflection on them.

LIMIT = 10 #: Integer
RATE = 0.25
DEBUG = false

class Account
  attr_reader :balance #: Float
  attr_accessor :count #: Integer

  #: (?Float) -> void
  def initialize(balance = 0.0)
    @balance = balance
    @count = 0
    @open = true #: bool
  end

  #: (Float) -> Float
  def deposit(amt)
    @balance += amt
    @count += 1
    @balance
  end

  #: () -> bool
  def open? = @open

  #: () -> void
  def toggle
    @open = !@open
  end
end

#: (?Integer, ?Float, ?bool) -> String
def defaults(n = 3, f = 1.5, b = true) = "#{n} #{f} #{b}"

puts "-- constants"
puts LIMIT.inspect, (LIMIT * 2).inspect, RATE.inspect, (RATE * 4.0).inspect, DEBUG.inspect, (!DEBUG).inspect

puts "-- ivars"
acct = Account.new
puts acct.deposit(10.5).inspect, acct.deposit(RATE).inspect, acct.count.inspect, acct.balance.inspect
acct.count = 42
puts acct.count.inspect, acct.open?.inspect
acct.toggle
puts acct.open?.inspect
acct.toggle
puts acct.open?.inspect, Account.new(2.0).balance.inspect

puts "-- default arguments"
puts defaults, defaults(4), defaults(5, 0.5), defaults(6, -1.0, false)

puts "-- multiple assignment"
a, b = 1, 2.5
puts a.inspect, b.inspect
i, j = 3, 4
i, j = j, i
puts i.inspect, j.inspect
lo, hi = [9, 2].sort
puts "#{lo} #{hi}"
p1, p2, p3 = 1, true, 0.5
puts p1.inspect, p2.inspect, p3.inspect

puts "-- respond_to?, send, class"
puts 1.respond_to?(:+).inspect, 1.respond_to?(:upcase).inspect, 1.5.respond_to?(:floor).inspect, true.respond_to?(:&).inspect, 1.5.respond_to?(:even?).inspect
puts 1.send(:+, 2).inspect, 3.public_send(:abs).inspect, 2.5.send(:floor).inspect, -4.send(:abs).inspect, true.send(:^, true).inspect
puts 1.class.inspect, 2.5.class.inspect, 2.5.class.name.inspect, 1.class.name.inspect, Integer.name.inspect, Float.inspect
