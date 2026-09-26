# skip: a local first assigned inside a block is hoisted to a `var` at the top of the enclosing Go function instead of the block body, so a value from one iteration or call leaks into the next (prints "first" twice; MRI prints nil the second time)

# rbs_inline: enabled

# each local joins nil + T to T? (its first assignment is nil), so only the
# hoisting scope is under test, not the zero value of a never-assigned local
[1, 2].each do |v|
  if v > 5
    mark = nil
  elsif v == 1
    mark = "first"
  end
  puts mark.inspect
end

out = [1, 2, 3].map do |v|
  seen = nil if v > 100
  seen = v * 10 if v.odd?
  seen.inspect
end
puts out.inspect

#: (Array[String]) -> void
def tags(words)
  words.each do |w|
    tag = nil if w.empty?
    tag = "long" if w.size > 3
    puts "#{w}: #{tag.inspect}"
  end
end
tags(["ruby", "go", "rust", "c"])
