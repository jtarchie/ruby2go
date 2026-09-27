# rbs_inline: enabled

# A local or a branch holding both Integer and Float is untyped, as MRI has it.
x = 1
x = 2.5
puts x
c = true #: bool
d = false #: bool
puts (c ? 1 : 2.5).inspect, (d ? 1 : 2.5).inspect
y = d ? 2.5 : 1
puts (y + 1).inspect
