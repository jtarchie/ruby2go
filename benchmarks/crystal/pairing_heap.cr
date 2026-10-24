# Crystal port of benchmarks/pairing_heap.rb, for the README comparison column.
# The pairing_heap gem (3.1.2) is ported below: same node layout, meld, two-pass merge_pairs and
# order proc, trimmed to the methods the benchmark calls. Dijkstra uses Int64: (cell * 7919 + n * 104_729)
# overflows Int32 (Ruby's Integer is unbounded).

module PairingHeap
  module MergePairs
    # Non-recursive two-pass merge: pair left to right, then fold right to left.
    def merge_pairs(heaps)
      return nil if heaps.nil?
      return heaps if heaps.next_sibling.nil?

      pairs = nil
      left = heaps
      while left
        right = left.next_sibling
        unless right
          left.next_sibling = pairs
          pairs = left
          break
        end
        next_val = right.next_sibling
        right = meld(left, right)
        right.next_sibling = pairs
        pairs = right

        left = next_val
      end

      left = pairs.not_nil!
      right = left.next_sibling
      while right
        next_val = right.next_sibling
        left = meld(left, right)
        right = next_val
      end
      left
    end
  end

  class PairingHeap(E, P)
    class Node(E, P)
      property elem : E
      property priority : P
      property subheaps : Node(E, P)?
      property parent : Node(E, P)?
      property prev_sibling : Node(E, P)?
      property next_sibling : Node(E, P)?

      def initialize(@elem, @priority)
      end

      def remove_from_parents_list!
        if prev = prev_sibling
          prev.next_sibling = next_sibling
          next_sibling.try &.prev_sibling = prev
        else # parent.subheaps must equal self
          parent.not_nil!.subheaps = next_sibling
          next_sibling.try &.prev_sibling = nil
        end
        self.prev_sibling = nil
        self.next_sibling = nil
      end
    end

    include MergePairs

    @root : Node(E, P)?

    def initialize(&@order : P, P -> Bool)
      @root = nil
      @nodes = {} of E => Node(E, P)
    end

    def push(elem : E, priority : P)
      raise ArgumentError.new("Element already in the heap") if @nodes.has_key?(elem)

      node = Node(E, P).new(elem, priority)
      @nodes[elem] = node
      @root = if root = @root
                meld(root, node)
              else
                node
              end
      self
    end

    def empty?
      @root.nil?
    end

    def pop : E?
      root = @root
      return nil if root.nil?

      elem = root.elem
      @nodes.delete(elem)
      @root = new_root = merge_pairs(root.subheaps)
      if new_root
        new_root.parent = nil
        new_root.next_sibling = nil
        new_root.prev_sibling = nil
      end
      elem
    end

    def pop_with_priority : {E?, P?}
      node = @root
      pop
      {node.try(&.elem), node.try(&.priority)}
    end

    def change_priority(elem : E, priority : P)
      node = @nodes[elem]?
      raise ArgumentError.new("Provided element is not in heap") if node.nil?
      unless @order.call(priority, node.priority)
        raise ArgumentError.new("Priority cannot be changed to a less prioritary value.")
      end

      node.priority = priority
      parent = node.parent
      return self if parent.nil?
      return self if @order.call(parent.priority, node.priority)

      node.remove_from_parents_list!
      root = meld(node, @root.not_nil!)
      root.parent = nil
      @root = root
      self
    end

    def include?(key : E)
      @nodes.has_key?(key)
    end

    private def meld(left : Node(E, P), right : Node(E, P)) : Node(E, P)
      if @order.call(left.priority, right.priority)
        parent = left
        child = right
      else
        parent = right
        child = left
      end
      child.next_sibling = parent.subheaps
      parent.subheaps = child
      child.next_sibling.try &.prev_sibling = child
      child.prev_sibling = nil
      child.parent = parent
      parent
    end
  end

  class SimplePairingHeap(E, P)
    class Node(E, P)
      property elem : E
      property priority : P
      property subheaps : Node(E, P)?
      property next_sibling : Node(E, P)?

      def initialize(@elem, @priority)
      end
    end

    include MergePairs

    @root : Node(E, P)?
    getter size = 0

    def initialize(&@order : P, P -> Bool)
      @root = nil
    end

    def push(elem : E, priority : P)
      node = Node(E, P).new(elem, priority)
      @root = if root = @root
                meld(root, node)
              else
                node
              end
      @size += 1
      self
    end

    def any?
      !@root.nil?
    end

    def pop : E?
      root = @root
      return nil if root.nil?
      @size -= 1

      elem = root.elem
      @root = merge_pairs(root.subheaps)
      @root.try &.next_sibling = nil

      elem
    end

    private def meld(left : Node(E, P), right : Node(E, P)) : Node(E, P)
      if @order.call(left.priority, right.priority)
        parent = left
        child = right
      else
        parent = right
        child = left
      end
      child.next_sibling = parent.subheaps
      parent.subheaps = child
      parent
    end
  end

  # Priority queue where the smallest priority is the most prioritary
  class MinPriorityQueue(E, P) < PairingHeap(E, P)
    def initialize
      super { |a, b| a <= b }
    end

    def decrease_key(elem : E, priority : P)
      change_priority(elem, priority)
    end
  end
end

SIDE = 150_i64

def neighbors(cell : Int64) : Array(Int64)
  row, col = cell.divmod(SIDE)
  out = [] of Int64
  out << cell - SIDE if row > 0
  out << cell + SIDE if row < SIDE - 1
  out << cell - 1 if col > 0
  out << cell + 1 if col < SIDE - 1
  out
end

dist = {0_i64 => 0_i64}
queue = PairingHeap::MinPriorityQueue(Int64, Int64).new
queue.push(0_i64, 0_i64)
until queue.empty?
  cell, d = queue.pop_with_priority
  next unless cell && d

  neighbors(cell).each do |n|
    alt = d + (cell * 7919 + n * 104_729) % 97 + 1
    known = dist[n]?
    next if known && known <= alt

    dist[n] = alt
    queue.include?(n) ? queue.decrease_key(n, alt) : queue.push(n, alt)
  end
end
puts dist[SIDE * SIDE - 1]
puts dist.values.sum

heap = PairingHeap::SimplePairingHeap(Int32, Int32).new { |a, b| a <= b }
100_000.times { |i| heap.push((i * 7919) % 100_003, (i * 7919) % 100_003) }
sorted = 0
prev = -1
while heap.any?
  x = heap.pop
  raise "out of order" unless x && x >= prev

  prev = x
  sorted += 1
end
puts sorted
