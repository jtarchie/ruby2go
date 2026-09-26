# skip: `next` inside a begin/ensure in a closure block (a method that rescues around yield) returns only from the begin's Go func literal, so the rest of the block still runs ("after begin 2" is printed)

# rbs_inline: enabled

#: () { (Integer) -> void } -> void
def guarded
  [1, 2, 3].each { |g| yield g }
rescue => e
  puts "guarded caught #{e.message}"
end

guarded do |g|
  begin
    next if g == 2
    puts "in begin #{g}"
  ensure
    puts "ensure #{g}"
  end
  puts "after begin #{g}"
end

guarded do |g|
  begin
    raise "odd" if g.odd?
  rescue
    next
  end
  puts "even #{g}"
end
