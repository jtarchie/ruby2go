# skip: reopening a class with the same superclass spelled differently (`< ::A` after `< A`, `M::K < M::Base` after `K < Base` inside M) is a compile error "class C reopened with a different superclass": collectClass compares the superclass source text, not the resolved class

# rbs_inline: enabled

class A
end

class C < A
  #: () -> String
  def one = "one"
end

class C < ::A
  #: () -> String
  def two = "two"
end

module M
  class Base
  end

  class K < Base
  end
end

class M::K < M::Base
  #: () -> String
  def three = "three"
end

puts C.new.one, C.new.two, M::K.new.three
