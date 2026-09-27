
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
