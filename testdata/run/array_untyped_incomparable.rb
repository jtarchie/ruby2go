# rbs_inline: enabled

#: (untyped) -> untyped
def ident(v) = v

# <=> on untyped values answers nil when they do not compare.
puts (ident(1) <=> "a").inspect, (ident(:a) <=> 1).inspect
mixed = []
mixed << 3
mixed << "x"
# Which pair MRI names depends on its sort order, so only the prefix is checked.
begin
  mixed.sort
rescue ArgumentError => e
  puts e.message.start_with?("comparison of ")
end
begin
  mixed.max
rescue ArgumentError => e
  puts e.message.end_with?(" failed")
end
