# rbs_inline: enabled

class Fatal < Exception; end

[1, 2, 3].each do |i|
  puts "item #{i}"
  begin
    raise Fatal, "not a StandardError" if i == 2
  rescue => e
    puts "never #{e.message}"
  end
end
puts "unreachable"
