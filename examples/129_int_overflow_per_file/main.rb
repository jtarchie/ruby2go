# Each file's `# int_overflow:` comment governs only the arithmetic written in that file (decision 381):
# checksum.rb wraps like C, factorial.rb promotes to Bignums, and this file keeps the default.
require_relative "checksum"
require_relative "factorial"

words = %w[alpha beta gamma delta]
words.each do |w|
  h = checksum(w)
  # the low 64 bits as two 32-bit halves read the same on MRI's Bignum and a wrapped Integer
  puts "#{w}: #{(h >> 32) & 0xFFFF_FFFF} #{h & 0xFFFF_FFFF}"
end

puts "30! = #{factorial(30)}"
puts "digits in 50!: #{factorial(50).size}"

total = 0
words.each { |w| total += w.size }
puts "letters: #{total}"
