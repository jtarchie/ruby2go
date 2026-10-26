# rbs_inline: enabled

# zip with a block, or chained into one, walks both Arrays by index without building the pairs (decision 390).

names = %w[ann bob cy dee]
scores = [91, 78, 85]
bonus = [5, 0, 10, 2]

names.zip(scores) { |name, score| puts "#{name}: #{score || "absent"}" }

total = 0
scores.zip(bonus).each { |s, b| total += s + (b || 0) }
puts "total with bonus: #{total}"

puts names.zip(scores).count { |_n, s| s && s > 80 }
puts names.zip(scores).map { |n, s| "#{n.upcase}=#{s.to_i}" }.join(" ")
puts scores.zip(bonus).sum { |s, b| s * (b || 1) }
p names.zip(scores, bonus)
