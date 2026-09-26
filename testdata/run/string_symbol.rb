# rbs_inline: enabled

# Decision 23: Symbol is its own type, never equal to the String of the same name.
a = :apple
puts (a <=> :banana).inspect, (a <=> :apple).inspect, (:b <=> :a).inspect, (:a <=> :ab).inspect
puts (a == :apple).inspect, (a == "apple").inspect, (a != :x).inspect, ("apple" == a).inspect
puts (a.to_s == "apple").inspect, (a == "apple".to_sym).inspect

puts a.to_s.inspect, a.name.inspect, a.to_sym.inspect, a.inspect
puts a.size.inspect, :"héllo".size.inspect, :"".size.inspect
puts (a.hash == :apple.hash).inspect, (a.hash == :pear.hash).inspect

# Comparable on Symbol
puts (a < :b).inspect, (a >= :apple).inspect, (:z > :a).inspect, (:a <= :a).inspect
puts a.between?(:a, :b).inspect, :zz.clamp(:a, :m).inspect, :b.clamp(:a, :m).inspect

# Symbol#inspect quotes only names that would not read back bare.
syms = [:abc, :"a b", :a?, :b!, :c=, :[], :[]=, :+, :-, :-@, :+@, :<=>, :==, :===, :=~] #: Array[Symbol]
syms += [:!, :!=, :!~, :<<, :>>, :<=, :>=, :**, :%, :/, :*, :&, :|, :^, :~, :<, :>]
syms += [:"foo-bar", :"9a", :"", :A, :Abc?, :_x, :"a?b", :"a==", :"?", :"=", :"[]?", :call, :"a\nb", :"q\"q"]
syms.each { |s| puts s.inspect }
puts syms.size

# to_s gives the bare name, whatever inspect does.
puts :"a b".to_s, :+.to_s, :"q\"q".to_s.inspect

# Sorting, min/max and &:sym blocks.
puts [:b, :a, :c].sort.inspect, [:b, :a].max.inspect, [:b, :a].min.inspect
puts [:x, :y].map(&:to_s).inspect, %w[p q].map(&:to_sym).inspect, %i[up down].map(&:size).inspect

# case/when matches symbols by identity of name.
[:pear, :apple, :fig].each do |fruit|
  case fruit
  when :pear then puts "pear"
  when :apple then puts "apple!"
  else puts "other #{fruit}"
  end
end

# Symbols as hash keys; Hash#inspect prints them as labels (decision 34).
config = { port: 8080, host: "localhost" } #: Hash[Symbol, untyped]
puts config[:port], config[:host], config.keys.inspect, config[:nope].inspect
counts = {} #: Hash[Symbol, Integer]
[:a, :b, :a].each { |k| counts[k] = (counts[k] || 0) + 1 }
puts counts.inspect, counts.key?(:a).inspect

# f(a: 1) on a method without keyword parameters passes a Hash (decision 23).

#: (Hash[Symbol, String]) -> String
def greet(opts) = "hi #{opts[:name]}"
puts greet(name: "x"), greet(:name => "y"), greet({ name: "z" })

puts :sym.frozen?.inspect, :sym.nil?.inspect, "#{:interp}"

# Symbols inside tuples (sort keys), tally/group_by/min_by, and as Hash values.
pairs = [[:b, 1], [:a, 2], [:a, 1]]
puts pairs.sort.inspect, pairs.map(&:first).tally.inspect, pairs.max.inspect
sp = [["b", :x], ["a", :y]]
puts sp.sort.inspect
by_name = { "a" => :x, "b c" => :"y z" } #: Hash[String, Symbol]
puts by_name.inspect, by_name.values.inspect
ss = %i[b a c]
puts ss.sort.reverse.inspect, ss.map(&:to_s).map(&:upcase).inspect, ss.min_by(&:to_s).inspect, ss.include?(:a).inspect
puts ss.sort_by { |x| x.to_s }.inspect, ss.group_by(&:size).inspect
