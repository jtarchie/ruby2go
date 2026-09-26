# skip: the 0d decimal prefix is copied into Go source, which does not parse (missing ',' in argument list)
# rbs_inline: enabled

puts 0d17.inspect, 0D0.inspect
