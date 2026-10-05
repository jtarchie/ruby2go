# rbs_inline: enabled

# Observable (MRI's `require "observer"`): the observers and the dirty flag
# are instance variables, as in MRI's observer.rb (@observer_peers maps each
# observer to the method notify_observers calls, @observer_state), held in
# each includer's struct (decision 147); notify_observers dispatches with
# plain Ruby `send` (decision 32).
module Observable
  # @rbs @observer_peers: Hash[untyped, Symbol]?
  # @rbs @observer_state: bool?

  #: (untyped, ?Symbol) -> void
  def add_observer(observer, fn = :update)
    unless observer.respond_to?(fn)
      raise NoMethodError, "observer does not respond to `#{fn}'"
    end

    peers = @observer_peers || {} #: Hash[untyped, Symbol]
    peers[observer] = fn
    @observer_peers = peers
  end

  #: (untyped) -> void
  def delete_observer(observer)
    @observer_peers&.delete(observer)
  end

  #: () -> void
  def delete_observers
    @observer_peers&.clear
  end

  #: () -> Integer
  def count_observers = @observer_peers&.size || 0

  #: (?bool) -> void
  def changed(state = true)
    @observer_state = state
  end

  #: () -> bool
  def changed? = @observer_state || false

  #: (*untyped) -> void
  def notify_observers(*args)
    return unless changed?

    (@observer_peers || {}).each do |observer, fn|
      __observable_send(observer, fn, args) if observer.respond_to?(fn)
    end
    changed(false)
  end

  private

  #: (untyped, Symbol, Array[untyped]) -> void
  def __observable_send(observer, fn, args) = %x{
    rbSendByName(observer, string(fn), rbFCall, (*args)...)
    return
  }
end
