# skip: an array literal where Array[T]? or a tuple? is expected ignores the expectation (genArray only looks through TClass/TTuple), so `return [r, c]` for [Integer, Integer]? and fetch(k, []) fail go build

# rbs_inline: enabled
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
