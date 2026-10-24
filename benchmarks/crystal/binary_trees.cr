# Crystal port of benchmarks/binary_trees.rb, for the README comparison column.
# The Computer Language Benchmarks Game
# Binary Trees - class-based nodes

class Node
  property left : Node?
  property right : Node?

  def initialize(@left, @right)
  end
end

def make_tree(depth)
  if depth == 0
    return Node.new(nil, nil)
  end
  d = depth - 1
  Node.new(make_tree(d), make_tree(d))
end

def check_tree(node : Node) : Int32
  left = node.left
  right = node.right
  if left.nil? || right.nil?
    return 1
  end
  1 + check_tree(left) + check_tree(right)
end

max_depth = (ARGV[0]? || "10").to_i
min_depth = 4
if min_depth + 2 > max_depth
  max_depth = min_depth + 2
end

stretch_depth = max_depth + 1
stretch_tree = make_tree(stretch_depth)
puts check_tree(stretch_tree)

long_lived_tree = make_tree(max_depth)

depth = min_depth
while depth <= max_depth
  iterations = 2 ** (max_depth - depth + min_depth)
  check = 0
  i = 1
  while i <= iterations
    t = make_tree(depth)
    check = check + check_tree(t)
    i = i + 1
  end
  puts check
  depth = depth + 2
end

puts check_tree(long_lived_tree)
