class Greeter
  attr_accessor :name

  def initialize(name)
    @name = name
  end

  def greet
    "hello #{@name}"
  end
end
