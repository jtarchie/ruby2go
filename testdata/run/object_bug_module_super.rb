# skip: super inside a module method (to the includer's superclass) is a compile error "super: no parent method hi"

# rbs_inline: enabled

module Loud
  #: () -> String
  def hi = "loud " + super
end

class A
  #: () -> String
  def hi = "a"
end

class B < A
  include Loud
end

puts B.new.hi
