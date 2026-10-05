# rbs_inline: enabled

# Marshal (decision 137): rb2go's own bytes (prelude/go/marshal.go), never MRI's; what round-trips is the object graph.
module Marshal
  MAJOR_VERSION = 4 #: Integer
  MINOR_VERSION = 8 #: Integer

  #: (untyped) -> String
  def self.dump(obj) = %x{ rbMarshalDump(obj, -1) }

  #: (untyped, untyped) -> untyped
  # @dynamic
  def self.__dump_2(obj, arg)
    return __dump_limit(obj, arg) if arg.is_a?(Integer)
    arg.write(dump(obj))
    arg
  end

  #: (untyped, untyped, untyped) -> untyped
  # @dynamic
  def self.__dump_3(obj, io, limit)
    io.write(__dump_limit(obj, limit))
    io
  end

  #: (untyped, untyped) -> String
  def self.__dump_limit(obj, limit) = %x{ rbMarshalDump(obj, int(rbAs[Integer](limit, "Integer"))) }

  #: (untyped) -> untyped
  # @dynamic
  def self.load(source)
    return __load(source, nil) if source.is_a?(String)
    return __load_io(source) if __exact_io?(source)
    __load(source.read, nil)
  end

  #: (untyped) -> untyped
  def self.restore(source) = load(source)

  #: (untyped, untyped) -> untyped
  def self.__load(head, body) = %x{ rbMarshalLoad(head, body) }

  #: (untyped) -> bool
  def self.__exact_io?(source) = %x{
    _, ok := rbMarshalReadN(source, 0)
    return Boolean(ok)
  }

  #: (untyped) -> untyped
  def self.__load_io(source) = %x{
    head, _ := rbMarshalReadN(source, len(rbMarshalMagic)+8)
    body, _ := rbMarshalReadN(source, rbMarshalBodySize(head))
    return rbMarshalLoad(head, body)
  }
end
