# rbs_inline: enabled

# OpenStruct (decision 118): fields made up as they are assigned, held in an
# ordered Hash. A field read or write compiles to method_missing (decision
# 31), so `person.name` is untyped and `person.email = x` stores x.
class OpenStruct < Object
  #: (?Hash[untyped, untyped]) -> void
  def initialize(hash = {})
    @table = {} #: Hash[Symbol, untyped]
    hash.each { |k, v| @table[k.to_s.to_sym] = v }
  end

  #: (Symbol, *untyped) -> untyped
  def method_missing(name, *args)
    n = name.to_s
    return @table[n.delete_suffix("=").to_sym] = args.first if n.end_with?("=")

    @table[name]
  end

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = @table.key?(name.to_s.delete_suffix("=").to_sym)

  #: (untyped) -> untyped
  def [](name) = @table[name.to_s.to_sym]

  #: (untyped, untyped) -> untyped
  def []=(name, value)
    @table[name.to_s.to_sym] = value
  end

  #: () -> Hash[Symbol, untyped]
  def to_h
    out = {} #: Hash[Symbol, untyped]
    @table.each { |k, v| out[k] = v }
    out
  end

  #: () { (Symbol, untyped) -> void } -> void
  def each_pair
    @table.each { |k, v| yield k, v }
  end

  # The field's value, then each further key looked up in it (nil along the way stops).
  # @dynamic
  #: (untyped, *untyped) -> untyped
  def dig(name, *rest)
    v = self[name]
    rest.each do |k|
      return nil if v.nil?

      v = v[k]
    end
    v
  end

  #: (untyped) -> untyped
  def delete_field(name)
    key = name.to_s.to_sym
    raise NameError, "no field '#{key}' in #{inspect}" unless @table.key?(key)

    @table.delete(key)
  end

  #: (untyped) -> bool
  def ==(other)
    return false unless other.is_a?(OpenStruct)

    o = other #: OpenStruct
    o.to_h == @table
  end

  #: () -> String
  def inspect
    return "#<#{self.class.name}>" if @table.empty?

    "#<#{self.class.name} #{@table.map { |k, v| "#{k}=#{v.inspect}" }.join(", ")}>"
  end

  #: () -> String
  def to_s = inspect
end
