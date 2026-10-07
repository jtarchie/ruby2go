# rbs_inline: enabled

# GC.start is runtime.GC(); core, as MRI's (Benchmark.bmbm and WeakRef tests call it).
module GC
  #: () -> void
  def self.start = %x{ runtime.GC() }
end
