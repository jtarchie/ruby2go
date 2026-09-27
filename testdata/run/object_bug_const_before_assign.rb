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
