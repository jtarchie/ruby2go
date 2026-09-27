# rbs_inline: enabled

puts 1e15.inspect, 1.5e15.inspect, -1.2e15.inspect, 1234567890123456.0.to_s, 9999999999999998.0.inspect
puts 9_007_199_254_740_993.to_f.inspect
puts "#{2e15}"
# Non-integral values in [1e15, 1e16) stay in fixed form, as do integral ones below 1e15.
puts 1234567890123456.7.inspect, 999999999999999.9.inspect, 100000000000000.0.inspect
