# skip: Regexp#=== matches Symbols in MRI (/b/ === :abc is true); rb2go only matches String

# rbs_inline: enabled

puts(/b/ === :abc, /z/ === :abc)
case :abc
when /b/ then puts "symbol matched"
else puts "symbol not matched"
end
