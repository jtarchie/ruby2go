# rbs_inline: enabled

i = 0
seen = [] #: Array[Integer]
while i < 6
  i += 1
  begin
    next if i == 2
    break if i == 5
    raise "odd" if i.odd?
    seen << i
  rescue
    seen << -i
  end
end
puts seen.inspect, i

out = [] #: Array[Integer]
[1, 2, 3].each do |x|
  begin
    raise "two" if x == 2
    out << x
  rescue
    next
  ensure
    out << 0
  end
  out << 10
end
puts out.inspect
