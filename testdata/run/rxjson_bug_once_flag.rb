# skip: the /o flag is ignored, so /#{x}/o is re-interpolated on every evaluation; MRI interpolates once and keeps the first pattern

# rbs_inline: enabled

["a", "b"].each do |x|
  re = /#{x}/o
  puts re.source, re.match?("b")
end
