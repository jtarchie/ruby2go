# load_path: lib
require "demo"

puts Demo::Thing.new("hi").greet
puts Demo::Lazy.new.go
