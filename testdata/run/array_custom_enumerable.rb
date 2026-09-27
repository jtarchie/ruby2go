# rbs_inline: enabled

class Countdown
  include Enumerable #[Integer]

  #: (Integer) -> void
  def initialize(from)
    @from = from
  end

  #: () { (Integer) -> void } -> void
  def each
    n = @from
    while n > 0
      yield n
      n -= 1
    end
  end
end

class Shelf
  include Enumerable #[String]

  #: (*String) -> void
  def initialize(*titles)
    @titles = titles
  end

  #: () { (String) -> void } -> void
  def each(&block) = @titles.each(&block)
end

module Digits
  extend Enumerable #[Integer]

  #: () { (Integer) -> void } -> void
  def self.each(&block) = [3, 1, 2].each(&block)
end

class Version
  attr_reader :major #: Integer
  attr_reader :minor #: Integer

  #: (Integer, Integer) -> void
  def initialize(major, minor)
    @major = major
    @minor = minor
  end

  #: (Version) -> Integer
  def <=>(other) = major == other.major ? minor <=> other.minor : major <=> other.major

  #: () -> String
  def to_s = "v#{major}.#{minor}"

  #: () -> String
  def inspect = "#<V #{self}>"
end

class Nums
  include Enumerable #[Integer]

  #: (*Integer) -> void
  def initialize(*ns)
    @ns = ns
  end

  #: () { (Integer) -> void } -> void
  def each(&block) = @ns.each(&block)

  #: () -> Integer
  def total = reduce(0) { |a, b| a + b }

  #: () -> Array[Integer]
  def evens = select(&:even?)
end

class MoreNums < Nums
  #: () -> Integer
  def biggest = max || 0

  #: () -> String
  def summary = "#{count}:#{sort.inspect}:#{include?(8)}:#{first(1).inspect}"
end

class Pairs
  include Enumerable #[[String, Integer]]

  #: () -> void
  def initialize
    @data = [["a", 1], ["b", 2]] #: Array[[String, Integer]]
  end

  #: () { ([String, Integer]) -> void } -> void
  def each(&block) = @data.each(&block)
end

class Naturals
  include Enumerable #[Integer]

  #: () { (Integer) -> void } -> void
  def each
    n = 0
    while true
      yield n
      n += 1
    end
  end
end

class Tolerant
  include Enumerable #[Integer]

  # unannotated, and it rescues around yield: a closure, adapted for Enumerable
  def each
    n = 0
    while true
      n += 1
      begin
        yield n
      rescue ZeroDivisionError
        puts "tolerant #{n}"
      ensure
        puts "ensure #{n}" if n == 2
      end
      return if n == 3
    end
  end
end

cd = Countdown.new(4)
cd.each { |n| puts "cd #{n}" }
cd.each do |n|
  next if n == 3
  break if n == 1
  puts "cd2 #{n}"
end
puts cd.map { |n| n * n }.inspect, cd.select(&:even?).inspect, cd.reject(&:even?).inspect, cd.to_a.inspect
puts cd.include?(3).inspect, cd.include?(9).inspect, cd.count, cd.sort.inspect, cd.min.inspect, cd.max.inspect
puts cd.reduce(1) { |a, n| a * n }, cd.inject(0) { |a, n| a - n }, cd.first(2).inspect, cd.take(0).inspect, cd.first(9).inspect
puts cd.tally.inspect, cd.find { |n| n < 3 }.inspect, cd.detect { |n| n > 9 }.inspect
puts cd.sort_by { |n| [n % 2, n.to_s] }.inspect, cd.group_by(&:odd?).inspect, cd.flat_map { |n| [n, -n] }.inspect
puts cd.min_by { |n| (n - 2).abs }.inspect, cd.max_by { |n| -n }.inspect
puts cd.any? { |n| n > 3 }.inspect, cd.all?(&:positive?).inspect, cd.none? { |n| n > 3 }.inspect
cd.each_with_index { |n, i| puts "#{i}:#{n}" }
cd.each_with_index do |n, i|
  break if i == 2
  puts "ewi #{n}"
end
cd.each_entry { |n| print "e", n }
puts
zero = Countdown.new(0)
puts zero.to_a.inspect, zero.min.inspect, zero.max.inspect, zero.count, zero.first(1).inspect, zero.all? { |n| n > 9 }.inspect
puts zero.tally.inspect, zero.sort.inspect, zero.map { |n| n }.inspect, zero.min_by { |n| n }.inspect

shelf = Shelf.new("Dune", "Emma", "Ulysses", "Beloved")
puts shelf.sort.inspect, shelf.map(&:size).inspect, shelf.max_by(&:size).inspect, shelf.include?("Emma").inspect
puts shelf.select { |t| t.include?("e") }.inspect, shelf.first(2).inspect, shelf.count
puts Shelf.new.to_a.inspect, Shelf.new.max.inspect

puts Digits.sort.inspect, Digits.map { |d| d * 2 }.inspect, Digits.max.inspect, Digits.min.inspect, Digits.include?(2).inspect
puts Digits.to_a.inspect, Digits.reduce(0) { |a, d| a + d }, Digits.sort_by { |d| -d }.inspect, Digits.count

vs = [Version.new(2, 0), Version.new(1, 9), Version.new(1, 10)] #: Array[Version]
puts vs.sort.inspect, vs.min.inspect, vs.max.inspect
puts vs.sort_by { |v| -v.minor }.map(&:to_s).inspect
puts vs.map(&:to_s).join(" < ")
puts vs.min_by(&:minor).inspect, vs.max_by(&:major).inspect
puts vs.group_by(&:major).inspect
puts vs

mn = MoreNums.new(3, 8, 5)
puts mn.total, mn.evens.inspect, mn.biggest, mn.sort.inspect, mn.map { |x| x + 1 }.inspect
puts mn.summary, mn.to_a.inspect, mn.is_a?(Nums), mn.is_a?(Enumerable)
mn.each { |x| print x, "," }
puts
puts MoreNums.new.biggest, MoreNums.new.summary

prs = Pairs.new
prs.each { |k, v| puts "#{k}=#{v}" }
puts prs.map { |k, v| k * v }.inspect, prs.to_a.inspect, prs.sort_by { |_k, v| -v }.inspect
puts prs.find { |_k, v| v > 1 }.inspect, prs.min_by { |_k, v| v }.inspect, prs.include?(["b", 2])
puts prs.sort.inspect, prs.max.inspect, prs.group_by { |k, _v| k }.inspect, prs.tally.inspect
prs.each_with_index { |pr, i| puts "#{i}:#{pr.inspect}" }

nat = Naturals.new
puts nat.first(3).inspect, nat.take(2).inspect, nat.find { |n| n * n > 50 }.inspect, nat.include?(5)
puts nat.any? { |n| n > 3 }, nat.all? { |n| n < 3 }, nat.none? { |n| n == 4 }, nat.detect(&:positive?).inspect
nat.each do |n|
  break if n > 2
  puts n
end
nat.each_with_index do |n, i|
  break if i >= 2
  puts "#{i}:#{n}"
end

tol = Tolerant.new
tol.each { |n| puts 6 / (n - 2) }
puts tol.find(&:even?).inspect, tol.map { |n| n * 2 }.inspect, tol.include?(2)
