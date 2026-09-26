# skip: a user method named plus/neg/... and the operator +/-@ both get the Go name Plus/Neg, so calling both on untyped values (or any computed send) emits rbDynPlus / rbDynNeg twice: go build fails "redeclared"; MRI runs it

# rbs_inline: enabled

class Vec
  #: (Integer) -> Integer
  def plus(n) = n + 100

  #: () -> Integer
  def neg = -1
end

#: (untyped) -> untyped
def ident(v) = v

puts ident(1) + 2
puts ident(Vec.new).plus(1)
puts Vec.new.send(["neg"].first(1)[0] || "").inspect
