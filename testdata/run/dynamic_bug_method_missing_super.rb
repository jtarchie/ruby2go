# rbs_inline: enabled

class Picky
  #: (Symbol, *untyped) -> String
  def method_missing(name, *args)
    return "ok #{name}" if name.to_s.start_with?("get_")
    super
  end

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = name.to_s.start_with?("get_") || super
end

pk = Picky.new
puts pk.get_x, pk.respond_to?(:get_y), pk.respond_to?(:other)
begin
  pk.other
rescue NoMethodError => e
  puts e.message
end
