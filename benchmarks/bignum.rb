# int_overflow: promote
# rbs_inline: enabled

# Integers past 64 bits (decision 372): Fibonacci, factorials and digits of pi, all exact.

def fib_digit_sum(n)
  a = 0
  b = 1
  n.times { a, b = b, a + b }
  a.digits.sum
end

def factorial(n)
  f = 1
  i = 2
  while i <= n
    f *= i
    i += 1
  end
  f
end

def pi_digits(count)
  q = 1
  r = 0
  t = 1
  k = 1
  n = 3
  l = 3
  out = ""
  while out.size < count
    if 4 * q + r - t < n * t
      out += n.to_s
      nr = 10 * (r - n * t)
      n = (10 * (3 * q + r)) / t - 10 * n
      q *= 10
      r = nr
    else
      nr = (2 * q + r) * l
      nn = (q * (7 * k) + 2 + (r * l)) / (t * l)
      q *= k
      t *= l
      l += 2
      k += 1
      n = nn
      r = nr
    end
  end
  out
end

total = 0
200.times { |i| total += fib_digit_sum(500 + i) }
puts total
puts factorial(2000).to_s.size
digits = pi_digits(1200)
puts digits[0, 20], digits[-20, 20]
