# skip: implicit-argument `super` in an exception subclass's initialize passes String where Exception#initialize takes String?, so go build fails

# rbs_inline: enabled

class Plain < StandardError
  #: (String) -> void
  def initialize(msg)
    super
  end
end

class DefaultMsg < StandardError
  #: (?String) -> void
  def initialize(msg = "default message")
    super
  end
end

puts Plain.new("x").message
begin
  raise DefaultMsg
rescue DefaultMsg => e
  puts e.message
end
