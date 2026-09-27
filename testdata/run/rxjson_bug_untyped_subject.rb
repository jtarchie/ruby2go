# rbs_inline: enabled

#: () -> untyped
def nothing = nil

#: () -> untyped
def sym = :cat

ur = /a/ #: untyped
begin
  puts ur.match?(nothing), (ur =~ nothing).inspect, ur.match(nothing).inspect
rescue StandardError => e
  puts "dynamic nil: #{e.class}"
end
begin
  puts(/a/.match?(nothing), (/a/ =~ nothing).inspect, /a/.match(nothing).inspect)
rescue StandardError => e
  puts "typed nil: #{e.class}"
end
begin
  puts(/a/.match?(sym), (/a/ =~ sym).inspect, /(a)/.match(sym).inspect, "cat" =~ /#{sym}/)
rescue StandardError => e
  puts "symbol: #{e.class}"
end
