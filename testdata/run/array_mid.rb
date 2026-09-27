# rbs_inline: enabled

# each re-reads the length, so pushes, deletes and clear mid-loop match MRI
grow = [1, 2] #: Array[Integer]
grow.each do |x|
  grow << x * 10 if x < 10
  puts x
end
puts grow.inspect
shrink = [1, 2, 3, 4] #: Array[Integer]
seen = [] #: Array[Integer]
shrink.each do |x|
  seen << x
  shrink.delete(x)
end
puts seen.inspect, shrink.inspect
cleared = [1, 2] #: Array[Integer]
cleared.each do |x|
  cleared.clear
  puts x
end

# a rescue inside a closure-taking each catches the block's own raise
class Safe
  include Enumerable #[Integer]

  #: () { (Integer) -> void } -> void
  def each
    [1, 2].each do |x|
      begin
        yield x
      rescue ZeroDivisionError
        puts "rescued"
      end
    end
  end
end

puts Safe.new.to_a.inspect
Safe.new.each { |x| puts 10 / (x - 1) }

# include? compares objects by identity and untyped elements by value
class Pt
  attr_reader :x #: Integer

  #: (Integer) -> void
  def initialize(x)
    @x = x
  end
end

pt1 = Pt.new(1)
pts = [pt1, Pt.new(2)] #: Array[Pt]
puts pts.include?(pt1), pts.include?(Pt.new(1))
mixed_inc = []
mixed_inc << 3
mixed_inc << "x"
puts mixed_inc.include?(3).inspect, mixed_inc.include?("x").inspect, mixed_inc.include?("y").inspect

# an array literal returned where a tuple? or Array? is expected

#: (Array[Array[Integer]], Integer) -> [Integer, Integer]?
def find_pos(grid, target)
  grid.each_with_index do |row, r|
    row.each_with_index do |v, c|
      return [r, c] if v == target
    end
  end
  nil
end

#: (Array[Integer]?) -> Integer
def count_of(list) = list ? list.size : -1

grid = [[1, 2, 3], [4, 5, 6]] #: Array[Array[Integer]]
puts find_pos(grid, 5).inspect, find_pos(grid, 9).inspect
puts count_of([]), count_of(nil), count_of([7])
groups = { "odd" => [1] } #: Hash[String, Array[Integer]]
puts groups.fetch("even", []).size

# <=> on untyped values answers nil when they do not compare

#: (untyped) -> untyped
def ident(v) = v

puts (ident(1) <=> "a").inspect, (ident(:a) <=> 1).inspect
mixed = []
mixed << 3
mixed << "x"
# Which pair MRI names depends on its sort order, so only the prefix is checked.
begin
  mixed.sort
rescue ArgumentError => cmp_err
  puts cmp_err.message.start_with?("comparison of ")
end
begin
  mixed.max
rescue ArgumentError => cmp_err
  puts cmp_err.message.end_with?(" failed")
end
