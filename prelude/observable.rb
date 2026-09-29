# rbs_inline: enabled

# Observable (MRI's `require "observer"`): peers/dirty-flag live in a Go-stdlib side table keyed by identity (prelude/go/observable.go), since a mixin module has no `@ivar` field to write; `notify_observers` still dispatches via plain Ruby `send` (decision 32), not duplicated in Go.
module Observable
  #: (untyped, Symbol) -> void
  def add_observer(observer, fn = :update)
    unless observer.respond_to?(fn)
      raise NoMethodError, "observer does not respond to `#{fn}'"
    end

    __observable_add_observer(observer, fn)
  end

  #: (untyped) -> void
  def delete_observer(observer) = %x{ rbObservableDeleteObserver(self, observer) }

  #: () -> void
  def delete_observers = %x{ rbObservableDeleteObservers(self) }

  #: () -> Integer
  def count_observers = %x{ Integer(rbObservableCountObservers(self)) }

  #: (bool) -> void
  def changed(state = true) = %x{ rbObservableSetChanged(self, bool(state)) }

  #: () -> bool
  def changed? = %x{ Boolean(rbObservableChanged(self)) }

  #: (*untyped) -> void
  def notify_observers(*args)
    return unless changed?

    __observable_observers.zip(__observable_funcs).each do |observer, fn|
      next unless fn
      next unless observer.respond_to?(fn)

      __observable_send(observer, fn, args)
    end
    changed(false)
  end

  private

  #: (untyped, Symbol) -> void
  def __observable_add_observer(observer, fn) = %x{ rbObservableAddObserver(self, observer, fn) }

  #: () -> Array[untyped]
  def __observable_observers = %x{ rbObservableObservers(self) }

  #: () -> Array[Symbol]
  def __observable_funcs = %x{ rbObservableFuncs(self) }

  #: (untyped, Symbol, Array[untyped]) -> void
  def __observable_send(observer, fn, args) = %x{
    rbSendByName(observer, string(fn), rbFCall, (*args)...)
    return
  }
end
