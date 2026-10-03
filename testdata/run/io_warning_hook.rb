# rbs_inline: enabled
# A reopened Warning.warn receives every Kernel#warn (decision 129); standalone, since the hook is program-wide.
# stderr: match

module Warning
  #: (String, ?category: Symbol?) -> nil
  def self.warn(msg, category: nil)
    puts "hook #{msg.inspect} #{category.inspect}"
  end
end

warn "plain"
warn "exp", category: :experimental
warn "dep", category: :deprecated
warn 1, [2, 3]
Warning[:deprecated] = true
warn "dep", category: :deprecated
