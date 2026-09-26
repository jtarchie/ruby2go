# skip: equal? on Strings compares Go data pointers, so a result that shares the receiver's bytes (strip/sub/center with nothing to do, "#{s}", Symbol#to_s, "".dup) is identical; MRI returns a new String

# frozen_string_literal: true

# rbs_inline: enabled

x = "ab"
puts x.strip.equal?(x).inspect, x.sub("z", "y").equal?(x).inspect, x.gsub("z", "y").equal?(x).inspect
puts x.center(1).equal?(x).inspect, x.chomp.equal?(x).inspect, x.downcase.equal?(x).inspect, x.tr("z", "y").equal?(x).inspect
puts x.split(",")[0].equal?(x).inspect, x.lines[0].equal?(x).inspect, (x * 1).equal?(x).inspect
puts "#{x}".equal?(x).inspect, :a.to_s.equal?(:a.to_s).inspect, x.to_sym.to_s.equal?(x).inspect
e = ""
puts e.equal?(e.dup).inspect
