# rbs_inline: enabled
# args: -v -h

# --help/-h is built in: it prints the help and exits 0 (decision 101).
require "optparse"

OptionParser.new do |o|
  o.on("-v", "--verbose", "Be chatty") { |v| puts "verbose #{v}" }
  o.on("-n", "--name NAME", "Who") { |v| puts "name #{v}" }
end.parse!
puts "not reached"
