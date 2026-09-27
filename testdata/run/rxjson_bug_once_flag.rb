# rbs_inline: enabled

["a", "b"].each do |x|
  re = /#{x}/o
  puts re.source, re.match?("b")
end
