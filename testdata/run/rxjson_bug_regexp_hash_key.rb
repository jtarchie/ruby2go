# skip: Regexp has no value hash/eql?: equal interpolated regexps are different Hash keys and uniq keeps both

# rbs_inline: enabled

x = "a"
h = { /a/ => 1 } #: Hash[Regexp, Integer]
puts h[/a/].inspect, h[/#{x}/].inspect
res = [/a/, /#{x}/, /b/] #: Array[Regexp]
puts res.uniq.size
