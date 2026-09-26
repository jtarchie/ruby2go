# rbs_inline: enabled
require "json"

# Decision 25: the json gem's escapes ("/" and non-ASCII as-is) and its fpconv float format.

puts "".to_json, "plain".to_json, "a\"b".to_json, "back\\slash".to_json, "a/b</script>".to_json
puts "\n\r\t\b\f".to_json, "\u0000\u0001\u0008\u000b\u000e\u001f".to_json, "\u007f\e".to_json
puts "héllo 日本 😀     ".to_json, "ümlaut\n".to_json
puts "\u2028\u2029".to_json, "\u00a0\u00ad".to_json, "\u{10FFFF}".to_json, "\u{1F600}".to_json, "\u{7f}".to_json
puts "multi\nline\ttext with \"quotes\" and \\ back".to_json
puts JSON.generate("str"), JSON.generate(""), JSON.generate("\u0002")

puts 0.to_json, -1.to_json, 42.to_json, 123456789012345678.to_json, -9223372036854775807.to_json
puts JSON.generate(7), JSON.generate(-0)

puts true.to_json, false.to_json, nil.to_json, JSON.generate(true), JSON.generate(false), JSON.generate(nil)
puts :sym.to_json, :"with space".to_json, :"quote\"sym".to_json, :"é".to_json, JSON.generate(:s)

puts 0.0.to_json, -0.0.to_json, 1.0.to_json, -1.5.to_json, 0.1.to_json, (0.1 + 0.2).to_json, (1.0 / 3).to_json, (2.0 / 3).to_json
puts JSON.generate(2.5), JSON.generate(-0.0), JSON.generate(1e20)
floats = [
  1.5e-12, 1e-12, 1.5e-11, 1e-11, 1.5e-10, 1e-10, 1.5e-9, 1e-9,
  1.5e-8, 1e-8, 1.5e-7, 1e-7, 1.5e-6, 1e-6, 1.5e-5, 1e-5,
  1.5e-4, 1e-4, 1.5e-3, 1e-3, 1.5e-2, 1e-2, 1.5e-1, 1e-1,
  1.5e0, 1e0, 1.5e1, 1e1, 1.5e2, 1e2, 1.5e3, 1e3,
  1.5e4, 1e4, 1.5e5, 1e5, 1.5e6, 1e6, 1.5e7, 1e7,
  1.5e8, 1e8, 1.5e9, 1e9, 1.5e10, 1e10, 1.5e11, 1e11,
  1.5e12, 1e12, 1.5e13, 1e13, 1.5e14, 1e14, 1.5e15, 1e15,
  1.5e16, 1e16, 1.5e17, 1e17, 1.5e18, 1e18, 1.5e19, 1e19,
  1.5e20, 1e20, 1.5e21, 1e21, 1.5e22, 1e22, 1.5e23, 1.5e24, 1e24,
  1.2345678901234567e-7, 1.2345678901234567e14, 1.2345678901234567e16, 1.2345678901234567e22, 0.2,
  123.456, 1e300, 1e-300, 5e-324, 2.2250738585072014e-308, 1.7976931348623157e308, 9007199254740993.0, 99999999999999.9,
  999999999999999.9, 0.00001234, 12345678.9, 3.14159, 100.0, 12.5
] #: Array[Float]
floats.each { |f| puts "#{f.to_json} #{(-f).to_json}" }
puts floats.to_json

# NaN and Infinity have no JSON form: JSON::GeneratorError, a StandardError.
nan = 0.0 / 0.0
inf = 1.0 / 0.0
[nan, inf, -inf].each do |f|
  begin
    puts f.to_json
  rescue JSON::GeneratorError => e
    puts "GeneratorError: #{e.message}"
  end
end
begin
  puts JSON.generate(-inf)
rescue StandardError => e
  puts "#{e.class} #{e.is_a?(JSON::GeneratorError)}"
end
begin
  puts [1.0, nan].to_json
rescue JSON::GeneratorError => e
  puts "in array: #{e.message}"
end
puts "before uncaught"
puts JSON.generate({ "x" => inf })
puts "not reached"
