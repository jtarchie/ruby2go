# rbs_inline: enabled

# Observable (MRI's `require "observer"`): the observers and the dirty flag
# are instance variables, as in MRI's observer.rb (@observer_peers maps each
# observer to the method notify_observers calls, @observer_state), held in
# each includer's struct (decision 147). The observer's type is inferred from
# the program's add_observer calls (decision 317), so notify_observers' `send`
# switches over those classes' names only (decision 316), not every name.
module Observable
  # @rbs @observer_state: bool?

  # @rbs fn: Symbol
  # @rbs return: void
  def add_observer(observer, fn = :update)
    unless observer.respond_to?(fn)
      raise NoMethodError, "observer does not respond to `#{fn}'"
    end

    if (peers = @observer_peers)
      peers[observer] = fn
    else
      @observer_peers = { observer => fn }
    end
  end

  # @rbs return: void
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

  # @dynamic
  #: (*untyped) -> void
  def notify_observers(*args)
    return unless changed?

    @observer_peers&.each do |observer, fn|
      observer.send(fn, *args) if observer.respond_to?(fn)
    end
    changed(false)
  end
end
