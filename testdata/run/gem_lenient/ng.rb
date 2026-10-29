class Ng
  def initialize
    @names = Hash.new
    @names["a"] = 1
    @args = Array.new
    @args << "x"
  end

  # a reset to nil: a gem's container ivar is optional after it (getoptlong's terminate)
  def reset
    @names = nil
    @args = nil
    self
  end

  def names = @names

  def args = @args

  # a nil in a multiple assignment types the local by its later writes (getoptlong's get)
  def multi
    name, arg = nil, ""
    name = "n" if @names
    [name, arg]
  end
end
