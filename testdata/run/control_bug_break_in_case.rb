# rbs_inline: enabled

i = 0
while i < 10
  i += 1
  case i
  when 3 then break
  end
end
puts i

out = [] #: Array[String]
[1, 2, 3].each do |x|
  case x
  when 2 then break
  else out << x.to_s
  end
end
puts out.inspect

vals = [1, "s", nil] #: Array[untyped]
n = 0
vals.each do |v|
  n += 1
  case v
  when String then break
  end
end
puts n
