# skip: a value typed Object is statically dispatched to Kernel#to_s/inspect ("#<String>") and is_a? folds to false; a literal argument is not wrapped (Go string, "#<string>"); Array[Object]#inspect panics

# rbs_inline: enabled

#: (Object) -> String
def desc(o) = "#{o}|#{o.inspect}|#{o.is_a?(String)}|#{o.is_a?(Symbol)}"

puts desc("a"), desc(:a), desc("a" + "b"), desc(1)
objs = ["s", :t, 2.5] #: Array[Object]
puts objs.map { |o| o.to_s }.inspect
puts objs.inspect
