# rbs_inline: enabled

# GC.start is runtime.GC(); core, as MRI's (Benchmark.bmbm and WeakRef tests call it).
module GC
  #: () -> void
  def self.start = %x{ runtime.GC() }
end

# ObjectSpace.define_finalizer records nothing: a finalizer MRI would run at GC or exit (patchelf's close-file proc)
# never runs. ponytail: run them at exit, as MRI does, once a program's finalizer has an effect to observe.
module ObjectSpace
  #: (untyped, ?untyped) -> Array[untyped]
  def self.define_finalizer(obj, aproc = nil) = [0, aproc]

  #: (untyped) -> untyped
  def self.undefine_finalizer(obj) = obj

  # MRI's WeakMap, as drb's DRbObjectSpace keeps object ids in it (decision 416).
  # ponytail: values are held strongly, so nothing is collected, and keys compare by eql? (an Integer id's identity);
  # weakref.rb's weak pointers are the upgrade when a program's map must shrink
  class WeakMap
    #: () -> void
    def initialize
      @h = {} #: Hash[untyped, untyped]
    end

    #: (untyped) -> untyped
    def [](key) = @h[key]

    #: (untyped, untyped) -> untyped
    def []=(key, value)
      @h[key] = value
    end

    #: (untyped) -> bool
    def key?(key) = @h.key?(key)

    #: (untyped) -> bool
    def include?(key) = @h.key?(key)

    #: (untyped) -> bool
    def member?(key) = @h.key?(key)

    #: (untyped) -> untyped
    def delete(key) = @h.delete(key)

    #: () -> Integer
    def size = @h.size

    #: () -> Integer
    def length = @h.size

    #: () -> Array[untyped]
    def keys = @h.keys

    #: () -> Array[untyped]
    def values = @h.values
  end
end
