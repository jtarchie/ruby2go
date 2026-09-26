# skip: Hash#inspect quotes symbol labels that MRI prints bare: emoji and other non-letter identifier characters, a leading non-ASCII digit

# rbs_inline: enabled

emo = { :"😀" => 2, :"a😀" => 3, :"😀?" => 23, :"٣" => 25 } #: Hash[Symbol, Integer]
puts emo.inspect
