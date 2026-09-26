# skip: Regexp#source keeps the \/ escape and inspect/to_s leave a bare / unescaped (MRI: source "a/b", inspect /a\/b/)

# rbs_inline: enabled

x = "b/c"
puts(/a\/b/.source)
puts %r{a/b}.inspect, %r{a/b}.to_s, %r{a/b}.source
puts(/a#{x}/.inspect, /a#{x}/.to_s, /a#{x}/.source)
w = "w"
puts %r{#{w}/x}.inspect, /#{w}\//.source, /#{w}\//.inspect
