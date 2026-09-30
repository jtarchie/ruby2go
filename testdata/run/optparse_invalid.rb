# rbs_inline: enabled
# args: --bogus

# An unknown switch left unrescued ends the program like any exception.
require "optparse"

OptionParser.new { |o| o.on("-v") { |_| nil } }.parse!
puts "not reached"
