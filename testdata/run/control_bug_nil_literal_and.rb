# skip: `nil && x` evaluates to x: genAnd treats a nil-typed left side as never falsy and returns the right side (prints 5 and "a"; MRI prints nil)

# rbs_inline: enabled

x = nil && 5
puts x.inspect
s = "a"
y = nil && s
puts y.inspect
