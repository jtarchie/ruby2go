# rbs_inline: enabled

x = "a"
h = { /a/ => 1 } #: Hash[Regexp, Integer]
puts h[/a/].inspect, h[/#{x}/].inspect
res = [/a/, /#{x}/, /b/] #: Array[Regexp]
puts res.uniq.size
