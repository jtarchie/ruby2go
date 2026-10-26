# int_overflow: promote

#: (Integer) -> String
def factorial(n)
  f = 1
  (1..n).each { |i| f *= i }
  f.to_s
end
