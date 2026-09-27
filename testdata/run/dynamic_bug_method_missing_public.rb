# rbs_inline: enabled

class Config
  #: (Symbol, *untyped) -> String
  def method_missing(name, *args) = "mm #{name}"

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = false
end

c = Config.new
puts c.respond_to?(:method_missing), c.respond_to?(:respond_to_missing?)
