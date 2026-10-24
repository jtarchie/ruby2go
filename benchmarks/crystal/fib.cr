# Crystal port of benchmarks/fib.rb, for the README comparison column.

def fib(n : Int32) : Int32
  n < 2 ? n : fib(n - 1) + fib(n - 2)
end

puts fib(32)
