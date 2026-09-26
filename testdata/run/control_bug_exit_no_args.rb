# skip: a bare `exit` crashes the compiler (slice bounds out of range): the default `status = 0` of Kernel#exit is read from main.rb's source at the prelude's offset; in a longer file it reads a stray character instead (go build: undefined: p)

# rbs_inline: enabled

puts "before bare exit"
exit
puts "unreachable"
