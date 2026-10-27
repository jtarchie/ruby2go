# rbs_inline: enabled
# nil_check: runtime

# The magic comment above lets a possibly-nil value pass where a value is expected, checked when it runs, as MRI does (decision 393).

prices = { "apple" => 3, "pear" => 5, "plum" => 2 }
stock = [4, 0, 7]

%w[apple pear kiwi plum].each_with_index do |fruit, i|
  price = prices[fruit] # Integer?: a missing key is nil
  count = stock[i]      # Integer?: past the end is nil
  total = price * count # without the comment: a compile error until both are narrowed
  puts "#{fruit}: #{total}"
rescue NoMethodError, TypeError => e
  puts "#{fruit}: #{e.class}: #{e.message}"
end

rate = { "pear" => 2.5 }["apple"]
begin
  puts 10 + rate
rescue TypeError => e
  puts "10 + rate: #{e.message}"
end
