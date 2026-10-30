# prime.rb's shapes (decision 417): an iterator's value with a valueless break in its block.
class Primeish
  def witness?(r, x, m)
    return false if r.times do
      x = (x * x) % 97
      break if x == m
    end

    true
  end

  def scan(xs, stop)
    xs.each do |x|
      break if x == stop
    end
  end
end
