# rbs_inline: enabled

require "securerandom"

puts SecureRandom.uuid_v4.match?(/\A\h{8}-\h{4}-4\h{3}-[89ab]\h{3}-\h{12}\z/)

v7 = SecureRandom.uuid_v7
puts v7.match?(/\A\h{8}-\h{4}-7\h{3}-[89ab]\h{3}-\h{12}\z/)
v7b = SecureRandom.uuid_v7
puts v7 != v7b

puts SecureRandom.alphanumeric(20).match?(/\A[A-Za-z0-9]{20}\z/)
puts SecureRandom.alphanumeric(10, chars: ["x", "y", "z"]).match?(/\A[xyz]{10}\z/)
puts SecureRandom.alphanumeric(8, chars: ["ab", "cd"]).length
