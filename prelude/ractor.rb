# rbs_inline: enabled
#
# Ractor (decision 103): an isolated goroutine with a port. Ractor.new's
# block is checked for isolation at compile time (internal/compiler/ractor.go),
# messages and constructor arguments are deep-copied (rbRactorCopy), and
# Ractor.receive/current/main? resolve lexically to the enclosing block's
# ractor, or to main at the top level.

# @go_type struct { port *Ractor_Port; done chan struct{}; val any; err any; name *String; id int }
class Ractor < Object
  class Error < RuntimeError; end
  class IsolationError < Error; end
  class ClosedError < StopIteration; end
  class MovedError < Error; end
  class UnsafeError < Error; end

  # value/join raise it around the exception that ended the ractor, its cause.
  class RemoteError < Error
    #: (Ractor, Exception) -> void
    def initialize(ractor, cause)
      @message = "thrown by remote Ractor."
      @ractor = ractor
      @cause = cause
    end

    #: () -> Ractor
    def ractor = @ractor
  end

  # An unbounded message queue; every port shares one lock so Ractor.select can wait on several.
  # @go_type struct { q rbPort }
  class Port < Object
    #: () -> Port
    def self.new = %x{ return &Ractor_Port{} }

    # The message is deep-copied first; a frozen Array/Hash is shared, as MRI shares shareable objects.
    #: (untyped) -> self
    def <<(obj) = %x{
      self.q.send(rbRactorCopy(obj))
      return self
    }

    #: (untyped) -> self
    def send(obj) = self << obj

    # `move: true` is a plain send: the sender keeps a usable object, where MRI leaves a MovedObject.
    #: (untyped, Hash[Symbol, bool]) -> self
    def __send_2(obj, opts) = self << obj

    #: () -> untyped
    def receive = %x{ return self.q.receive() }

    #: () -> void
    def close = %x{ self.q.close() }

    #: () -> bool
    def closed? = %x{ Boolean(self.q.isClosed()) }

    #: () -> String
    def inspect = %x{ return String("#<Ractor::Port>") }
  end

  # Ractor.new compiles to a hidden `rbNewRactor()` local plus one of these, by argument count; arguments are copied into the block.
  #: (Ractor) { () -> untyped } -> Ractor
  def self.__start(r) = %x{ return r.start(func() any { return blk() }) }

  # @rbs [A] (Ractor, A) { (A) -> untyped } -> Ractor
  def self.__start_2(r, a) = %x{
    a = rbCopyAs(a, map[any]any{})
    return r.start(func() any { return blk(a) })
  }

  # @rbs [A, B] (Ractor, A, B) { (A, B) -> untyped } -> Ractor
  def self.__start_3(r, a, b) = %x{
    seen := map[any]any{}
    a, b = rbCopyAs(a, seen), rbCopyAs(b, seen)
    return r.start(func() any { return blk(a, b) })
  }

  # @rbs [A, B, C] (Ractor, A, B, C) { (A, B, C) -> untyped } -> Ractor
  def self.__start_4(r, a, b, c) = %x{
    seen := map[any]any{}
    a, b, c = rbCopyAs(a, seen), rbCopyAs(b, seen), rbCopyAs(c, seen)
    return r.start(func() any { return blk(a, b, c) })
  }

  # `Ractor.new(name: "w") { }`
  #: (Ractor, Hash[Symbol, String]) { () -> untyped } -> Ractor
  def self.__start_named(r, opts) = %x{
    r.name = Hash_Op_idx(opts, Symbol("name"))
    return r.start(func() any { return blk() })
  }

  #: () -> Ractor
  def self.main = %x{ return rbMainRactor }

  # Main plus the ractors still running.
  #: () -> Integer
  def self.count = %x{ Integer(1 + rbRactorLive.Load()) }

  # The first port with a message, with it: `port, msg = Ractor.select(a, b)`.
  #: (*Port) -> [Port, untyped]
  def self.select(*ports) = %x{
    p, v := rbPortSelect(rest_)
    return Tuple2[*Ractor_Port, any]{F0: p, F1: v}
  }

  # The message is deep-copied first.
  #: (untyped) -> self
  def <<(obj) = %x{
    self.port.q.send(rbRactorCopy(obj))
    return self
  }

  #: (untyped) -> self
  def send(obj) = self << obj

  # `move: true` is a plain send (see Port#__send_2).
  #: (untyped, Hash[Symbol, bool]) -> self
  def __send_2(obj, opts) = self << obj

  # Ractor.receive inside this ractor's block.
  #: () -> untyped
  def __receive = %x{ return self.port.q.receive() }

  # Blocks until the block finishes; an exception there is re-raised as RemoteError with it as cause.
  #: () -> untyped
  def value = %x{
    <-self.done
    self.reraise()
    return self.val
  }

  #: () -> self
  def join = %x{
    <-self.done
    self.reraise()
    return self
  }

  #: () -> Port
  def default_port = %x{ return self.port }

  #: () -> String?
  def name = %x{ return self.name }

  #: () -> String
  def inspect = %x{
    status := "running"
    select {
    case <-self.done:
      status = "terminated"
    default:
    }
    return String(fmt.Sprintf("#<Ractor:#%d %s>", self.id, status))
  }

  #: () -> String
  def to_s = inspect
end
