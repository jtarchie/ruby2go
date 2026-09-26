# skip: a non-nilable String/Symbol in a condition emits `s != nil || true`, which go build rejects (mismatched types String and untyped nil)

# rbs_inline: enabled

s = "x"
puts "string is truthy" if s
e = ""
puts(e ? "empty string is truthy" : "falsy")
sym = :a
puts "symbol is truthy" if sym
