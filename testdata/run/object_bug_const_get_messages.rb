# rbs_inline: enabled

module M
end

begin
  M.const_get(42)
rescue TypeError => e
  puts "TypeError: #{e.message}"
end

begin
  M.const_get(nil)
rescue TypeError => e
  puts "TypeError: #{e.message}"
end

begin
  M.const_get(:lower)
rescue NameError => e
  puts "NameError: #{e.message}"
end

begin
  M.const_get("")
rescue NameError => e
  puts "NameError: #{e.message}"
end

begin
  puts M.const_defined?("lower").inspect
rescue NameError => e
  puts "NameError: #{e.message}"
end
