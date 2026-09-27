# rbs_inline: enabled

class Vault
  #: () -> String
  def open = "open"

  private

  #: () -> String
  def secret = "shh"
end

#: (untyped) -> untyped
def ident(v) = v

v = Vault.new
["open", "secret"].each { |m| puts v.send(m).inspect }
puts ident(v).send(:secret).inspect
