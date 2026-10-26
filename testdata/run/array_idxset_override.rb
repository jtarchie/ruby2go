# A program's own Array#[]= runs for every store, in bounds too: the in-bounds fast path (decision 387) is only the prelude's.
class Array
  def []=(i, v)
    puts "set #{i} #{v}"
  end
end

a = [1, 2, 3]
a[0] = 9
a[1] = 8
p a
