# rbs_inline: enabled

require "json"

begin
  puts "\xff".to_json
rescue JSON::GeneratorError => e
  puts "GeneratorError #{e.class} #{e.message}"
end
begin
  puts ["ok", "a\xffb"].to_json
rescue JSON::GeneratorError => e
  puts "in array: #{e.message}"
end
