# skip: nil passed dynamically into a `*untyped` rest parameter (e.g. method_missing's *args) raises TypeError "no implicit conversion of nil into untyped"; MRI passes nil

# rbs_inline: enabled

class Ghost
  #: (Symbol, *untyped) -> untyped
  def method_missing(name, *args) = "ghost #{name}(#{args.inspect})"
end

class Logger
  #: (*untyped) -> String
  def log(*parts) = parts.inspect
end

#: (untyped) -> untyped
def ident(v) = v

puts ident(Ghost.new).fly(1, nil)
puts ident(Logger.new).log(nil, 2)
