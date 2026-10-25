# int_overflow: promote
# rbs_inline: enabled

# Integers past 64 bits, as MRI grows them: this file's magic comment (or
# `rugaroo build --int-overflow=promote`) turns a growing local into a Bignum.

# Factorial: the product outgrows 64 bits at 21!.
f = 1
1.upto(30) { |i| f *= i }
puts "30! = #{f}"
puts "digits: #{f.to_s.size}, trailing zeros: #{f.digits.take_while(&:zero?).size}"

# Fibonacci: two locals feeding each other.
a = 0
b = 1
100.times { a, b = b, a + b }
puts "fib(100) = #{a}"

# Powers and literals past 64 bits.
big = 2 ** 100
puts "2**100 = #{big}"
puts "2**100 - 1 in hex: #{(big - 1).to_s(16)}"
puts "u128 max: #{340282366920938463463374607431768211455}"
puts "2**100 / 3 = #{big / 3}, remainder #{big % 3}"
puts "a Float: #{big.to_f}"

# Small values stay plain machine integers; comparisons mix freely.
puts "fib(100) > 2**64: #{a > 2 ** 64}"
puts "class: #{f.class}, even? #{f.even?}"

# Spigot digits of pi (Gibbons), all exact integer arithmetic.
q = 1
r = 0
t = 1
k = 1
n = 3
l = 3
digits = ""
while digits.size < 30
  if 4 * q + r - t < n * t
    digits += n.to_s
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
puts "pi: #{digits[0]}.#{digits[1..]}"
