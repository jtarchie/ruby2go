# rbs_inline: enabled

puts(/b/ === :abc, /z/ === :abc)
case :abc
when /b/ then puts "symbol matched"
else puts "symbol not matched"
end
