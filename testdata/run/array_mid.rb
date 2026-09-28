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
idx_arr = %w[a b c b]
p idx_arr.index("b"), idx_arr.index("z"), idx_arr.index { |x| x > "a" }, idx_arr.find_index("c"), idx_arr.find_index { |x| x == "q" }, idx_arr.rindex("b"), idx_arr.rindex("q")

pairs = [["a", "1"], ["b"]] #: Array[Array[String]]
pairs.each { |k, v| p [k, v] }
opt = [["x", nil]] #: Array[Array[String?]]
opt.each { |k, v| p k, v }
nums = [[1, 2, 3]] #: Array[Array[Integer]]
p nums.map { |a, b| a.to_i + b.to_i }

en_a = [5, 3, 8, 1, 9, 2]
en_e = [] #: Array[Integer]
p en_a.count { |x| x > 2 }, en_a.count(3), en_a.inject { |s, x| s + x }, en_a.reduce { |s, x| s * x }, en_e.inject { |s, x| s + x }
p en_a.each_with_object([]) { |x, acc| acc << x * 2 }, en_a.filter_map { |x| x * 10 if x.odd? }, en_a.partition(&:even?), en_a.minmax, en_e.minmax
p en_a.min(2), en_a.max(2), en_a.sort { |x, y| y <=> x }, en_a.take_while { |x| x > 2 }, en_a.drop_while { |x| x > 2 }, en_a.drop(4)
en_a.each_slice(4) { |s| p s }
p en_a.each_slice(4).to_a, en_a.each_cons(5).to_a, en_a.each_slice(2).map(&:sum)
en_a.each_cons(5) { |c| p c }
p en_a.zip(%w[a b c]), en_a.sort.chunk_while { |x, y| y == x + 1 }.to_a, en_a.slice_when { |x, y| y < x }.to_a
p en_a.each_with_index.map { |x, i| x * i }, en_a.find_all(&:odd?), en_a.filter { |x| x < 3 }
en_h = {a: 1, b: 2, c: 3}
p en_h.filter_map { |k, v| k if v.odd? }, en_h.each_with_object({}) { |(k, v), acc| acc[v] = k }, en_h.partition { |k, v| v > 1 }, en_h.min_by { |k, v| -v }
p en_h.count { |k, v| v > 1 }, en_h.sum { |k, v| v }, en_h.each_slice(2).to_a, en_h.sort_by { |k, v| -v }

ar_a = [1, 2, 3, 4, 2]
ar_b = [2, 4, 6]
p ar_a - ar_b, ar_a & ar_b, ar_a | ar_b, ar_a.rotate, ar_a.rotate(-1), ar_a.rotate(7), ar_a.values_at(0, 2, 9), ar_a.intersect?(ar_b), ar_a.union([9]), ar_a.difference([1])
p [1, 2, 3].combination(2).to_a, [1, 2, 3].permutation(2).to_a.size, [1, 2].product([3, 4]), [1, 2, 3].permutation.first(2), [1, 2].combination(0).to_a
p [1, 3, 5, 7, 9].bsearch { |x| x >= 4 }, [1, 3].bsearch { |x| x > 9 }, ar_a.count(2)
ar_c = [5, 1, 4]
ar_c.sort!
p ar_c.dup
ar_c.map! { |x| x * 2 }
p ar_c.dup, ar_c.select! { |x| x > 2 }.inspect, ar_c.select! { |x| x > 2 }.inspect, ar_c.reject! { |x| x > 9 }.inspect, ar_c.reject! { |x| x > 99 }.inspect
ar_c.delete_if { |x| x == 8 }
p ar_c
ar_c.keep_if(&:positive?)
p ar_c
ar_d = [3, 1, 3, 2]
p ar_d.uniq!.inspect, ar_d.uniq!.inspect
ar_d.reverse!
p ar_d.dup, ar_d.insert(1, 9, 8).dup, ar_d.insert(-2, 7).dup, ar_d.each_index.to_a, ar_d.fill(0)
ar_e = [1, 2, 3]
p ar_e.each_with_index.to_a, ar_e.each.with_index(1).to_a, ar_e.map.with_index { |x, i| x * i }, ar_e.map.with_index(1) { |x, i| [i, x] }, ar_e.select.with_index { |x, i| i.even? }, ar_e.reject.with_index { |x, i| i.zero? }
ar_e.each.with_index(1) { |x, i| print x, i, " " }
ar_e.each_index { |i| print i }
puts
ar_w = %w[b a c]
ar_w.sort_by! { |s| s }
p ar_w

me_nums = [1, 2] #: Array[Integer]
puts me_nums.map.inspect
p me_nums.select, me_nums.reject.to_a
