# rbs_inline: enabled

z = 0 #: Integer
begin
  puts (z ** -1).inspect
rescue ZeroDivisionError => e
  puts e.message
end
puts "uncaught next"
puts (0 ** -2).inspect
