# rbs_inline: enabled

emo = { :"😀" => 2, :"a😀" => 3, :"😀?" => 23, :"٣" => 25 } #: Hash[Symbol, Integer]
puts emo.inspect
mix = { é: 1, "@iv": 1, "$g": 1, "a=": 1, A: 1, a?: 1, "+": 1, "[]": 1, "1a": 1, "a b": 1, "`": 1 } #: Hash[Symbol, Integer]
puts mix.inspect
