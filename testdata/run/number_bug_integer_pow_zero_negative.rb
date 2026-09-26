# skip: 0 ** -1 returns 1; MRI raises ZeroDivisionError (divided by 0)
# rbs_inline: enabled

z = 0 #: Integer
begin
  puts (z ** -1).inspect
rescue ZeroDivisionError => e
  puts e.message
end
puts "uncaught next"
puts (0 ** -2).inspect
