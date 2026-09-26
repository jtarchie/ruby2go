# skip: a user class named like a Go runtime helper (Opt, Ref) is emitted unmangled and go build fails with "Opt redeclared"

# rbs_inline: enabled

class Opt
  #: () -> String
  def to_s = "opt"
end

class Ref
  #: () -> String
  def to_s = "ref"
end

puts Opt.new, Ref.new
