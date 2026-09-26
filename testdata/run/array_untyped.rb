# rbs_inline: enabled

#: (untyped) -> String
def describe(x)
  if x.is_a?(Array)
    "array of #{x.size}: " + x.map { |e| describe(e) }.join(" ")
  elsif x.is_a?(Integer)
    "int(#{x})"
  elsif x.nil?
    "nil"
  else
    x.inspect
  end
end

#: (untyped) -> untyped
def ident(v) = v

#: (untyped) -> void
def push_two(x)
  x << 2
end

#: (untyped) -> String
def kind(x)
  case x
  when Array then "array #{x.size}"
  when Integer then "int"
  when nil then "nil"
  else "other"
  end
end

bag = []
bag << 3
bag << "x"
bag << nil
bag << :sym
bag << 2.5
bag << [1, "y"]
puts bag.inspect, bag.to_s, bag.size, bag.empty?
puts bag[0].inspect, bag[2].inspect, bag[-1].inspect, bag[99].inspect, bag.last.inspect
puts bag.first(5).join("-").inspect
puts bag.compact.inspect, bag.compact.size
puts bag.include?(nil).inspect
puts bag.map { |e| e.inspect }.inspect, bag.map { |e| e.to_s }.join("|")
puts bag.select { |e| e.nil? }.size, bag.reject { |e| e.nil? }.size
puts bag.count, bag.reverse.inspect, bag.first(2).inspect
puts describe(bag)
puts bag.delete(nil).inspect, bag.inspect, bag.delete("zzz").inspect
puts bag.delete("x").inspect, bag.inspect
puts bag.delete_at(0).inspect, bag.pop.inspect, bag.shift.inspect, bag.inspect

untyped_empty = []
puts untyped_empty.inspect, untyped_empty.size, untyped_empty.first(1).inspect, untyped_empty[0].inspect
untyped_empty.push("a")
untyped_empty.unshift(1)
untyped_empty.concat(["b", 2])
puts untyped_empty.inspect, (untyped_empty + [nil]).inspect

same = []
same << 3
same << 3
same << "3"
same << 3.0
puts same.uniq.inspect, same.tally.inspect, same.delete(3).inspect, same.inspect

row = [1, "a", :b, 2.5]
puts row.inspect, row.size, row[1].inspect, row.last.inspect
rows = [[1, "a"], [2, "b"]]
puts rows.inspect, rows.size

holder = [] #: Array[untyped]
holder << [1, 2]
holder << { k: "v" }
puts holder.inspect, describe(holder)

mixed_typed = [1, "a", nil, [2, [3]]] #: Array[untyped]
puts describe(mixed_typed)

dyn = ident([3, 1, 2])
puts dyn.size, dyn[0].inspect, dyn.last.inspect, dyn.include?(1), dyn.empty?, dyn.inspect, dyn.length
dyn << 4
puts dyn.inspect, dyn.join("-"), dyn == [3, 1, 2, 4], dyn.reverse.inspect
puts dyn.pop.inspect, dyn.shift.inspect, dyn.inspect, dyn.first(2).inspect
typed = [0] #: Array[Integer]
push_two(typed)
push_two(typed)
puts typed.inspect
puts kind(typed), kind([]), kind(3), kind(nil), kind("s"), kind([[1], [2]]), kind(holder)

mix = []
mix << 3
mix << "x"
mix << nil
mix << 2.5
puts mix.find { |e| e.nil? }.inspect, mix.any? { |e| e.nil? }, mix.all? { |e| e.nil? }, mix.none? { |e| e.nil? }
puts mix.group_by { |e| e.nil? }.inspect, mix.min_by { |e| e.to_s }.inspect, mix.max_by { |e| e.to_s.size }.inspect
puts mix.flat_map { |e| [e, e] }.size, mix.reject { |e| e.nil? }.inspect, mix.reduce(0) { |a, e| e.nil? ? a : a + 1 }
mix.each_with_index { |e, i| puts "#{i}=#{e.inspect}" }
mix.reverse_each { |e| print e.inspect, " " }
puts
puts mix.map(&:to_s).inspect, mix.map(&:inspect).join(","), mix.tally.size, mix.sort_by { |e| e.to_s }.inspect
