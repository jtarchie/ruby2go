# skip: Hash.new with an annotation is rejected ("undefined method new for singleton(Hash)"); genNew's @go_type path is unreachable

# rbs_inline: enabled

h = Hash.new #: Hash[String, Integer]
h["a"] = 1
puts h.inspect, h.size
