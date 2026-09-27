# rbs_inline: enabled

s = "x"
puts "string is truthy" if s
e = ""
puts(e ? "empty string is truthy" : "falsy")
sym = :a
puts "symbol is truthy" if sym
