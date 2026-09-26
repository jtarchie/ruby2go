# skip: reading a constant before its assignment has run gives the Go zero value (0) instead of raising NameError "uninitialized constant LIMIT"

# rbs_inline: enabled

#: () -> Integer
def lim = LIMIT

begin
  puts lim
rescue NameError => e
  puts "NameError: #{e.message}"
end
LIMIT = 5
puts lim
