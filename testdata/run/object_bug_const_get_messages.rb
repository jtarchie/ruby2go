# skip: const_get/const_defined? errors differ from Ruby 4: TypeError "42 is not a symbol nor a string" (MRI: "no implicit conversion of Integer into String"); a malformed name gives "uninitialized constant M::lower" / false (MRI: NameError "wrong constant name lower"); const_get("") returns nil instead of raising

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
