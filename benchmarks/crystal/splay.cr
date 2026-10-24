# Crystal port of benchmarks/splay.rb, for the README comparison column.

class SplayNode
  property key : Int32
  property value : Int32
  property left : SplayNode?
  property right : SplayNode?

  def initialize(@key, @value)
    @left = nil
    @right = nil
    @tag = "splay"
  end
end

class SplayTree
  @root : SplayNode?

  def initialize
    @root = nil
    @size = 0
    @tag = "tree"
  end

  def empty_p
    @root.nil?
  end

  def insert(key, value)
    root = @root
    if root.nil?
      @root = SplayNode.new(key, value)
    else
      root = splay(key)
      if root.key != key
        node = SplayNode.new(key, value)
        if key > root.key
          node.left = root
          node.right = root.right
          root.right = nil
        else
          node.right = root
          node.left = root.left
          root.left = nil
        end
        @root = node
      end
    end
    @size = @size + 1
  end

  def find(key)
    if empty_p
      -1
    else
      splay(key).key
    end
  end

  def splay(key) : SplayNode
    dummy = SplayNode.new(0, 0)
    sl = dummy
    sr = dummy
    t = @root.not_nil!
    while true
      if key < t.key
        l = t.left
        break if l.nil?
        if key < l.key
          y = l
          t.left = y.right
          y.right = t
          t = y
          break if t.left.nil?
        end
        sr.left = t
        sr = t
        t = t.left.not_nil!
      elsif key > t.key
        r = t.right
        break if r.nil?
        if key > r.key
          y = r
          t.right = y.left
          y.left = t
          t = y
          break if t.right.nil?
        end
        sl.right = t
        sl = t
        t = t.right.not_nil!
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
  idx = idx + 1
end

# Find known keys
found = 0
idx = 0
while idx < 25000
  key = (idx * 7919) % 100000
  result = tree.find(key)
  if result == key
    found = found + 1
  end
  idx = idx + 1
end

puts found
puts "done"
