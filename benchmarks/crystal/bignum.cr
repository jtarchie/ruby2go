# Crystal port of benchmarks/bignum.rb, for the README comparison column: BigInt (GMP) throughout.
require "big"

def fib_digit_sum(n : Int32) : Int32
  a = BigInt.new(0)
  b = BigInt.new(1)
  n.times { a, b = b, a + b }
  a.to_s.each_char.sum { |c| c.to_i }
end

def factorial(n : Int32) : BigInt
  f = BigInt.new(1)
  (2..n).each { |i| f *= i }
  f
end

def pi_digits(count : Int32) : String
  q = BigInt.new(1)
  r = BigInt.new(0)
  t = BigInt.new(1)
  k = 1
  n = BigInt.new(3)
  l = 3
  out = String::Builder.new
  size = 0
  while size < count
    if 4 * q + r - t < n * t
      s = n.to_s
      out << s
      size += s.size
      nr = 10 * (r - n * t)
      n = (10 * (3 * q + r)) // t - 10 * n
      q *= 10
      r = nr
    else
      nr = (2 * q + r) * l
      nn = (q * (7 * k) + 2 + (r * l)) // (t * l)
      q *= k
      t *= l
      l += 2
      k += 1
      n = nn
      r = nr
    end
  end
  out.to_s
end

total = 0
200.times { |i| total += fib_digit_sum(500 + i) }
puts total
puts factorial(2000).to_s.size
digits = pi_digits(1200)
puts digits[0, 20], digits[-20, 20]
