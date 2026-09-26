# rbs_inline: enabled

# Decision 23 at run time: a type switch tells String from Symbol although both are Go strings.
vals = ["str", :sym, 1, nil, "", :""] #: Array[untyped]
vals.each do |v|
  kind = case v
         when String then "String #{v.size}"
         when Symbol then "Symbol #{v.inspect}"
         when nil then "nil"
         else "other"
         end
  puts kind
end
vals.each { |v| puts "#{v.is_a?(String)} #{v.is_a?(Symbol)} #{v.inspect}" }
puts vals.map(&:to_s).inspect, vals.map(&:inspect).inspect

# Untyped Hash keys: "a" and :a are different keys.
h = {} #: Hash[untyped, Integer]
h["a"] = 1
h[:a] = 2
h["a"] = 3
puts h.size, h.inspect, h["a"].inspect, h[:a].inspect, h["b"].inspect
puts ["a", :a, "a", :a].uniq.inspect

# Decision 32: String methods called on an untyped value.
u = "Hello" #: untyped
puts u.upcase, u.size, u[1].inspect, (u + "!").inspect, u.split("l").inspect
puts u.start_with?("He").inspect, u.index("l").inspect, u.inspect, u == "Hello", u.equal?(u)
puts u.center(9).inspect, u.to_sym.inspect, u.chars.size, "#{u}!", u.to_s.downcase
s = :sym #: untyped
puts s.to_s, s.inspect, s.size, s == :sym, (s == "sym").inspect, s.to_sym.inspect, (s <=> :syn).inspect
puts "#{u} #{s}"

# Arity and argument types are checked like MRI; unknown methods raise NoMethodError.
begin
  u.reverse(1)
rescue ArgumentError => e
  puts "ArgumentError: #{e.message}"
end
begin
  u + 1
rescue TypeError
  puts "TypeError" # message wording: dynamic_bug_type_error_message
end
begin
  u.no_such_method
rescue NoMethodError
  puts "NoMethodError"
end

# respond_to?, send and class on literal receivers.
puts "abc".respond_to?(:upcase).inspect, "abc".respond_to?(:foo).inspect, :a.respond_to?(:size).inspect
puts "abc".send(:upcase), "abc".public_send(:center, 7).inspect, :a.send(:to_s), "x".send(:+, "y")
puts "a".class, "a".class.name, "a".is_a?(Comparable), "a".is_a?(Object), "a".is_a?(String), "a".kind_of?(Symbol)

# The same through a real `any` (a method returning untyped), not an annotated String local.

#: (untyped) -> untyped
def ident(v) = v

d = ident(" Hello\n")
puts (d * 2).inspect, d.between?("a", "z").inspect, d.clamp("A", "B").inspect
puts d.strip.inspect, d.chomp.inspect, d.lstrip.inspect, d.rstrip.inspect, d.reverse.inspect, d.capitalize.inspect, d.downcase.inspect
puts d.to_sym.inspect, d.nil?.inspect, d.dup.inspect, d[0].inspect, d[-1].inspect, d[99].inspect, d.index("z").inspect
puts d.split.inspect, d.split(nil).inspect, d.split("l").inspect, d.ljust(9).inspect, d.rjust(9).inspect, d.length, d.bytesize, d.empty?.inspect
puts d.lines.inspect, d.ord, d.include?("e").inspect, d.end_with?("\n").inspect, d.chars.size, d.sub("l", "L").inspect, d.gsub("l", "L").inspect
puts d.tr("lo", "01").inspect, "#{d.strip}!", (d == " Hello\n").inspect, (d != " Hello\n").inspect, (" Hello\n" == d).inspect
num = ident("42")
puts num.to_i + 1, num.to_f, (num.to_i * 2).inspect
ds = ident(:sy)
puts ds.to_s, ds.inspect, ds.size, ds.name, (ds == :sy).inspect, (ds == "sy").inspect, ds.to_sym.inspect, "#{ds}"
dn = ident(nil)
puts "[#{dn}]", dn.to_s.inspect, dn.inspect
begin
  d.center("x")
rescue TypeError
  puts "TypeError center"
end
begin
  d.split(1)
rescue TypeError
  puts "TypeError split"
end
begin
  d.sub("a")
rescue ArgumentError
  puts "ArgumentError sub"
end
