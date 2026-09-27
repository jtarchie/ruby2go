# rbs_inline: enabled

[1, 2].each do |z|
  w = z * 2
  puts w
end
[3].each do |z|
  w = "s#{z}"
  puts w
end

names = ["ann", "bo"] #: Array[String]
names.each { |n| s = n.upcase; puts s }
[4, 5].each { |c| s = c * 2; puts s }

#: () -> void
def later_outer
  labels = [1, 2].map do |z|
    item = z.to_s
    item
  end
  puts labels.inspect
  item = 5
  puts item + 1
end
later_outer
