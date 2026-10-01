# rbs_inline: enabled

# WeakRef (decision 110): a reference the garbage collector may clear. An
# object with identity (a struct class, decision 89's rbClassRefs) is held
# through Go's weak pointer to its first byte; a value (String, Integer)
# is held outright and is always alive, as MRI's immediates are. There is
# no method_missing: callers go through __getobj__, typed T.
# @rbs generic T
# @go_type struct { wp weak.Pointer[byte]; typ unsafe.Pointer; strong any; weak bool }
class WeakRef < Object
  class RefError < StandardError; end

  # @rbs [X] (X) -> WeakRef[X]
  def self.new(obj) = %x{
    r := &WeakRef[X]{}
    rbWeakSet(r, obj)
    return r
  }

  #: () -> bool?
  def weakref_alive? = %x{
    if !self.weak || self.wp.Value() != nil {
      return Ref(Boolean(true))
    }
    return nil
  }

  #: () -> T
  def __getobj__ = %x{ return rbWeakGet(self) }

  #: (T) -> T
  def __setobj__(obj)
    %x{
    rbWeakSet(self, obj)
    return obj
    }
  end

  #: () -> String
  def inspect = "#<WeakRef: #{weakref_alive? ? __getobj__.inspect : "(dead)"}>"
end
