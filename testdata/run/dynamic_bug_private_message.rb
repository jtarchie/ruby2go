# skip: calling a private method on an untyped receiver (or via public_send) says "undefined method 'secret' for ..."; MRI says "private method 'secret' called for an instance of Vault"

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

v = ident(Vault.new)
puts v.open
begin
  v.secret
rescue NoMethodError => e
  puts e.message
end
begin
  v.public_send(["secret"].first(1)[0] || "")
rescue NoMethodError => e
  puts e.message
end
