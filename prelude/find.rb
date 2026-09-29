# rbs_inline: enabled

module Find
  #: (*String) { (String) -> void } -> void
  def self.find(*paths) = %x{
    return func(yield func(String) bool) {
      rbFind(rest_, yield)
    }
  }

  #: () -> nil
  def self.prune = %x{
    findPruneRequested = true
  }
end
