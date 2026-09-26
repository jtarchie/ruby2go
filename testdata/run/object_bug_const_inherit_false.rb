# skip: the inherit=false argument of constants/const_get/const_defined? is ignored: inherited constants are still listed and found

# rbs_inline: enabled

class Parent
  COLOR = "red"
end

class Child < Parent
  SIZE = 5
end

puts Child.constants(false).inspect
puts Child.const_defined?(:COLOR, false).inspect
begin
  x = Child.const_get(:COLOR, false)
  puts x
rescue NameError => e
  puts "NameError: #{e.message}"
end
