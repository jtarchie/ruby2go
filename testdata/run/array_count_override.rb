# A program's own count/sum taking a block runs for every call: the loops at the call site (decision 402) are only the prelude's.
class Array
  #: () { (E) -> bool } -> Integer
  def count
    puts "count"
    n = 0
    each { |x| n += 1 if yield(x) }
    n
  end

  #: () { (E) -> Integer } -> Integer
  def sum
    puts "sum"
    t = 0
    each { |x| t += yield(x) }
    t
  end
end

a = [1, 2, 3]
n = a.count { |x| x > 1 }
s = a.sum { |x| x * 2 }
p [n, s]
