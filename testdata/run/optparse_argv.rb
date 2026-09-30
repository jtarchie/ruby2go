# rbs_inline: enabled
# args: build --jobs 4 -v src/main.rb --out=dist -- --literal

# parse! reads ARGV by default and leaves the operands in it (decision 101).
require "optparse"

jobs = 1
verbose = false
out = "." #: String
OptionParser.new do |o|
  o.on("-j", "--jobs N", Integer) { |n| jobs = n }
  o.on("-v", "--verbose") { |v| verbose = v }
  o.on("--out DIR") { |d| out = d }
end.parse!
p jobs, verbose, out, ARGV
