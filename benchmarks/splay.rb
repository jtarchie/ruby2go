# Splay tree benchmark (from yjit-bench / V8 Octane)

class SplayNode
  attr_accessor :key, :value, :left, :right

  def initialize(key, value)
    @key = key
    @value = value
    @left = nil
    @right = nil
    @tag = "splay"
  end
end

class SplayTree
  def initialize
    @root = nil
    @size = 0
    @tag = "tree"
  end

  def empty_p
    @root.nil?
  end

  def insert(key, value)
    if empty_p
      @root = SplayNode.new(key, value)
    else
      splay(key)
      if @root.key != key
        node = SplayNode.new(key, value)
        if key > @root.key
          node.left = @root
          node.right = @root.right
          @root.right = nil
        else
          node.right = @root
          node.left = @root.left
          @root.left = nil
        end
        @root = node
      end
    end
    @size += 1
  end

  def find(key)
    if empty_p
      -1
    else
      splay(key)
      @root.key
    end
  end

  def splay(key)
    dummy = SplayNode.new(0, 0)
    sl = dummy
    sr = dummy
    t = @root
    while true
      if key < t.key
        break if t.left.nil?
        if key < t.left.key
          y = t.left
          t.left = y.right
          y.right = t
          t = y
          break if t.left.nil?
        end
        sr.left = t
        sr = t
        t = t.left
      elsif key > t.key
        break if t.right.nil?
        if key > t.right.key
          y = t.right
          t.right = y.left
          y.left = t
          t = y
          break if t.right.nil?
        end
        sl.right = t
        sl = t
        t = t.right
      else
        break
      end
    end
    sl.right = t.left
    sr.left = t.right
    t.left = dummy.right
    t.right = dummy.left
    @root = t
  end
end

# Deterministic: insert keys 0..24999 in scrambled order
tree = SplayTree.new
idx = 0
while idx < 25000
  key = (idx * 7919) % 100000
  tree.insert(key, idx)
  idx += 1
end

# Find known keys
found = 0
idx = 0
while idx < 25000
  key = (idx * 7919) % 100000
  result = tree.find(key)
  found += 1 if result == key
  idx += 1
end

puts found
puts "done"
