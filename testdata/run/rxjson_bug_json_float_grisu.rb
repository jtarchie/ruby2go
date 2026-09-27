# rbs_inline: enabled

require "json"

puts 1e23.to_json, 1234567890123456.8.to_json, [1e23].to_json, JSON.generate(-1e23)
puts 5.326172664550231e-12.to_json, 8.92043287120562e+16.to_json
