# skip: adjacent literals where one is interpolated fail with "unsupported syntax: InterpolatedStringNode"

# rbs_inline: enabled

n = 1
puts "t" "u" "#{n}" 'v'
puts "total: #{n} " \
  "items"
