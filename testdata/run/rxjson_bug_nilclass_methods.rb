# skip: NilClass methods on a nil T? raise NoMethodError: an unmatched group's m[2].to_i / .to_f (MRI 0 / 0.0) and nil =~ /re/ (MRI nil)

# rbs_inline: enabled

m = /(\d+)(?:\.(\d+))?/.match("v12")
if m
  puts m[1].to_i
  puts m[2].to_i, m[2].to_f
end
line = nil #: String?
puts (line =~ /x/).inspect
