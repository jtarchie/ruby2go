# rbs_inline: enabled

x = 10
[1, 2].each { |x| puts x }
puts x
ys = [1, 2].map { |x| x * 3 }
puts ys.inspect, x

#: (Array[String]) -> String
def last_seen(items)
  item = "none"
  items.each do |item|
    puts "item #{item}"
  end
  item
end
puts last_seen(["a", "b"])
