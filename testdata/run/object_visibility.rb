# rbs_inline: enabled

# private/public sections, `private def`, self.private_method, subclasses calling inherited private methods.

#: (Integer) -> String
def top_helper(n) = "top#{n}"

class Vault
  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
    @pin = 1234
  end

  #: () -> String
  def open = "#{name} opened with #{masked} #{self.check(4)}"

  #: () -> String
  def via_top = top_helper(pin % 10)

  private

  #: () -> Integer
  def pin = @pin

  #: () -> String
  def masked = "*" * pin.to_s.size

  #: (Integer) -> bool
  def check(n) = pin.to_s.size == n

  public

  #: () -> String
  def status = "ok #{check(3)}"

  #: () -> String
  private def secret = "s3cret"

  #: () -> String
  def reveal = secret.reverse
end

class BigVault < Vault
  #: () -> String
  def deep = "#{masked}|#{pin}|#{secret}"

  def status = "big " + super
end

v = Vault.new("v")
puts v.open, v.status, v.reveal, v.via_top
b = BigVault.new("b")
puts b.open, b.status, b.deep, b.reveal
puts v.respond_to?(:open).inspect, v.respond_to?(:pin).inspect, v.respond_to?(:status).inspect
puts top_helper(-1)

# A private method called from inside a block, and a private hand-written setter through self.
class Dial
  #: () -> void
  def initialize
    @pin = 0
  end

  #: (Integer) -> Array[Integer]
  def scaled(n) = [1, 2].map { |x| helper(x) * n }

  #: (Integer) -> Integer
  def set(v)
    self.pin = v
    @pin
  end

  private

  #: (Integer) -> Integer
  def helper(x) = x + @pin

  #: (Integer) -> void
  def pin=(v)
    @pin = v
  end
end

d = Dial.new
puts d.scaled(1).inspect, d.set(5).inspect, d.scaled(10).inspect, d.respond_to?(:pin=).inspect
