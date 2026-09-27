# rbs_inline: enabled

class Vault
  #: () -> void
  def initialize
    @pin = 1234
    @hits = 0
  end

  #: () -> String
  def open
    self.hits = hits + 1
    "pin has #{pin.to_s.size} digits, hit #{hits}"
  end

  private

  attr_reader :pin #: Integer
  attr_accessor :hits #: Integer
end

v = Vault.new
puts v.open, v.open
