# rbs_inline: enabled

# klass.new through singleton(Base) where a subclass's initialize takes other arguments fails at run time (decision 19).
# Kept as print-and-compare: rb2go's error is a Go type assertion, not MRI's ArgumentError, so only stdout and exit code match.

class Base
  #: (Integer) -> void
  def initialize(n)
    @n = n
  end

  #: () -> Integer
  def n = @n
end

class Two < Base
  #: (Integer, Integer) -> void
  def initialize(a, b)
    super(a + b)
  end
end

class Same < Base
end

ks = [Base, Same, Two] #: Array[singleton(Base)]
ks.each { |k| puts "#{k.name}: #{k.new(1).n}" }
puts "unreachable"
