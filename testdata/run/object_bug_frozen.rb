# skip: Kernel#frozen? is always true; MRI says false for ordinary objects and Struct instances (true only for Data)

# rbs_inline: enabled

class Plain
end

Pair = Struct.new(:a, :b) #: [Integer, Integer]
Val = Data.define(:v) #: [Integer]

puts Plain.new.frozen?.inspect
puts Pair.new(1, 2).frozen?.inspect
puts Val.new(1).frozen?.inspect
