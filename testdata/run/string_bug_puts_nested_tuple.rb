# skip: puts prints a tuple (a 2-3 element mixed array, decision 22) as its inspect; MRI prints one element per line, nested ones too

# rbs_inline: enabled

puts [1, [2, [3]]]
t = [1, "a"]
puts t
