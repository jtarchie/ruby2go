# skip: the json gem's Grisu2 fpconv is not always shortest (1e23 -> 9.999999999999999e+22, 5.326172664550231e-12 -> 5.3261726645502314e-12); rb2go prints Go's shortest digits

# rbs_inline: enabled

require "json"

puts 1e23.to_json, 1234567890123456.8.to_json, [1e23].to_json, JSON.generate(-1e23)
puts 5.326172664550231e-12.to_json, 8.92043287120562e+16.to_json
