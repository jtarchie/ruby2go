# skip: `self.class` inside a module's instance method is the module's own class object, so it prints the module name; MRI gives the includer's class (Foo, Bar)

# rbs_inline: enabled

module Describe
  #: () -> String
  def kind = self.class.name

  #: () -> String
  def label = "#{self.class}!"
end

class Foo
  include Describe
end

class Bar < Foo
end

puts Foo.new.kind, Bar.new.kind, Bar.new.label
