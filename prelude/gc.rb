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
end
